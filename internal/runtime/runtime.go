//
// Copyright (c) 2019 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/edgexfoundry/app-functions-sdk-go/appcontext"
	"github.com/edgexfoundry/go-mod-core-contracts/clients"
	"github.com/edgexfoundry/go-mod-core-contracts/models"
	"github.com/edgexfoundry/go-mod-messaging/pkg/types"
	"github.com/ugorji/go/codec"
)

const unmarshalErrorMessage = "Unable to unmarshal message payload as %s"

// GolangRuntime represents the golang runtime environment
type GolangRuntime struct {
	TargetType    interface{}
	transforms    []appcontext.AppFunction
	isBusyCopying sync.Mutex
}

type MessageError struct {
	Err       error
	ErrorCode int
}

// ProcessMessage sends the contents of the message thru the functions pipeline
func (gr *GolangRuntime) ProcessMessage(edgexcontext *appcontext.Context, envelope types.MessageEnvelope) *MessageError {

	edgexcontext.LoggingClient.Debug("Processing message: " + strconv.Itoa(len(gr.transforms)) + " Transforms")

	if gr.TargetType == nil {
		gr.TargetType = &models.Event{}
	}

	if reflect.TypeOf(gr.TargetType).Kind() != reflect.Ptr {
		err := fmt.Errorf("TargetType must be a pointer, not a value of the target type.")
		edgexcontext.LoggingClient.Error(err.Error())
		return &MessageError{Err: err, ErrorCode: http.StatusInternalServerError}
	}

	// Must make a copy of the type so that data isn't retained between calls.
	target := reflect.New(reflect.ValueOf(gr.TargetType).Elem().Type()).Interface()

	// Only set when the data is binary so function receiving it knows how to deal with it.
	var contentType string

	switch target.(type) {
	case *[]byte:
		target = &envelope.Payload
		contentType = envelope.ContentType

	default:
		switch envelope.ContentType {
		case clients.ContentTypeJSON:

			if err := json.Unmarshal([]byte(envelope.Payload), target); err != nil {
				message := fmt.Sprintf(unmarshalErrorMessage, "JSON")
				edgexcontext.LoggingClient.Error(message, "error", err.Error(), clients.CorrelationHeader, envelope.CorrelationID)
				err = fmt.Errorf("%s : %s", message, err.Error())
				return &MessageError{Err: err, ErrorCode: http.StatusBadRequest}
			}

			event, ok := target.(*models.Event)
			if ok {
				// Needed for Marking event as handled
				edgexcontext.EventID = event.ID
			}

		case clients.ContentTypeCBOR:
			x := codec.CborHandle{}
			err := codec.NewDecoderBytes([]byte(envelope.Payload), &x).Decode(&target)
			if err != nil {
				message := fmt.Sprintf(unmarshalErrorMessage, "CBOR")
				edgexcontext.LoggingClient.Error(message, "error", err.Error(), clients.CorrelationHeader, envelope.CorrelationID)
				err = fmt.Errorf("%s : %s", message, err.Error())
				return &MessageError{Err: err, ErrorCode: http.StatusBadRequest}
			}

			// Needed for Marking event as handled
			edgexcontext.EventChecksum = envelope.Checksum

		default:
			message := "content type for input data not supported"
			edgexcontext.LoggingClient.Error(message, clients.ContentType, envelope.ContentType, clients.CorrelationHeader, envelope.CorrelationID)
			err := fmt.Errorf("'%s' %s", envelope.ContentType, message)
			return &MessageError{Err: err, ErrorCode: http.StatusBadRequest}
		}
	}

	edgexcontext.CorrelationID = envelope.CorrelationID

	// All functions expect an object, not a pointer to an object, so must use reflection to
	// dereference to pointer to the object
	target = reflect.ValueOf(target).Elem().Interface()

	var result interface{}
	var continuePipeline = true

	// Make copy of transform functions to avoid disruption of pipeline when updating the pipeline from registry
	gr.isBusyCopying.Lock()
	transforms := make([]appcontext.AppFunction, len(gr.transforms))
	copy(transforms, gr.transforms)
	gr.isBusyCopying.Unlock()

	for index, trxFunc := range transforms {
		if result != nil {
			continuePipeline, result = trxFunc(edgexcontext, result)
		} else {
			continuePipeline, result = trxFunc(edgexcontext, target, contentType)
		}
		if continuePipeline != true {
			if result != nil {
				if err, ok := result.(error); ok {
					edgexcontext.LoggingClient.Error(fmt.Sprintf("Pipeline function #%d resulted in error", index),
						"error", err.Error(), clients.CorrelationHeader, envelope.CorrelationID)
					return &MessageError{Err: err, ErrorCode: http.StatusUnprocessableEntity}
				}
			}
			break
		}
	}

	return nil
}

// SetTransforms is thread safe to set transforms
func (gr *GolangRuntime) SetTransforms(transforms []appcontext.AppFunction) {
	gr.isBusyCopying.Lock()
	gr.transforms = transforms
	gr.isBusyCopying.Unlock()
}
