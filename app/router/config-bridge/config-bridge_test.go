/*
 * Copyright 2022 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * imitations under the License.
 */

package config_bridge

import (
	"bytes"
	"encoding/json"
	testConstants "github.com/limechain/hedera-eth-bridge-validator/test/constants"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func Test_NewRouter(t *testing.T) {
	router := NewRouter(testConstants.ParserBridge)

	assert.NotNil(t, router)
}

func Test_configBridgeResponse(t *testing.T) {
	mocks.Setup()

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)

	var err error
	if err := enc.Encode(testConstants.ParserBridge); err != nil {
		http.Error(mocks.MResponseWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	bridgeConfigAsBytes := buf.Bytes()
	mocks.MResponseWriter.On("Header").Return(http.Header{})
	mocks.MResponseWriter.On("Write", bridgeConfigAsBytes).Return(len(bridgeConfigAsBytes), nil)

	bridgeResponseHandler := configBridgeResponse(testConstants.ParserBridge)
	bridgeResponseHandler(mocks.MResponseWriter, new(http.Request))

	assert.Nil(t, err)
	assert.NotNil(t, bridgeResponseHandler)
	assert.NotNil(t, bridgeConfigAsBytes)
}
