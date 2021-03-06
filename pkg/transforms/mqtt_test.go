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
package transforms

import (
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/models"
	"github.com/stretchr/testify/assert"
)

var addr models.Addressable

func init() {
	addr = models.Addressable{
		Address:   "localhost",
		Port:      1883,
		Protocol:  "tcp",
		Publisher: "",
		Password:  "",
		Topic:     "testMQTTTopic",
	}
}

func TestMQTTSend(t *testing.T) {
	t.SkipNow()

	mqttConfig := NewMqttConfig()
	sender := NewMQTTSender(lc, addr, nil, mqttConfig, false)

	dataToSend := "SOME DATA TO SEND"
	continuePipeline, result := sender.MQTTSend(context, dataToSend)
	assert.True(t, continuePipeline, "Should Continue Pipeline")
	assert.Equal(t, nil, result, "Should be nil")

}

func TestMQTTSendNoData(t *testing.T) {

	sender := MQTTSender{}
	continuePipeline, result := sender.MQTTSend(context)
	assert.False(t, continuePipeline, "Should Not Continue Pipeline")
	assert.Equal(t, "No Data Received", result.(error).Error(), "Error should be: No Data Received")

}

func TestMQTTSendInvalidData(t *testing.T) {
	t.SkipNow()

	expected := "passed in data must be of type []byte, string or implement json.Marshaler"
	mqttConfig := NewMqttConfig()
	sender := NewMQTTSender(lc, addr, nil, mqttConfig, false)

	type RandomObject struct {
		something string
	}

	dataToSend := RandomObject{something: "SOME DATA TO SEND"}
	continuePipeline, result := sender.MQTTSend(context, dataToSend)
	if !assert.False(t, continuePipeline, "Should Not Continue Pipeline") {
		t.Fatal()
	}
	assert.Equal(t, expected, result.(error).Error())

}

func TestMQTTSendPersistData(t *testing.T) {
	expected := "Could not connect to mqtt server"
	mqttConfig := NewMqttConfig()
	sender := NewMQTTSender(lc, addr, nil, mqttConfig, true)

	dataToSend := "Random data"
	continuePipeline, result := sender.MQTTSend(context, dataToSend)
	if !assert.False(t, continuePipeline, "Should Not Continue Pipeline") {
		t.Fatal()
	}
	assert.Contains(t, result.(error).Error(), expected)
	assert.NotNil(t, context.RetryData)
}

func TestNewMQTTSender(t *testing.T) {
	addr1 := models.Addressable{
		Address:   "localhost",
		Port:      1883,
		Protocol:  "tcp",
		Path:      "/path",
		Publisher: "publisher",
		User:      "user",
		Password:  "password",
		Topic:     "testMQTTTopic",
	}

	mqttConfig := NewMqttConfig()
	sender := NewMQTTSender(lc, addr1, nil, mqttConfig, false)
	assert.NotNil(t, sender.client, "Client should not be nil")
	opts := sender.client.OptionsReader()
	assert.Equal(t, "testMQTTTopic", sender.topic, "Topic should be set to testMQTTTopic")
	servers := opts.Servers()
	assert.Equal(t, 1, len(servers), "Should have 1 server connection defined")
	assert.Equal(t, "localhost:1883", servers[0].Host, "Server connection to host should be: localhost:1883")
	assert.Equal(t, "tcp", servers[0].Scheme, "Server connection protocol should be tcp")
	assert.Equal(t, "/path", servers[0].Path, "Server connection path should be path")
	assert.Equal(t, "publisher", opts.ClientID(), "ClientID should be publisher")
	assert.Equal(t, "user", opts.Username(), "Username should be user")
	assert.Equal(t, "password", opts.Password(), "Password should be password")
	assert.False(t, opts.AutoReconnect(), "Autoreconnect should be false")
}
