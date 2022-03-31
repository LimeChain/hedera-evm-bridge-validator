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
 * limitations under the License.
 */

package persistence

import (
	"github.com/limechain/hedera-eth-bridge-validator/test/helper"
	"github.com/limechain/hedera-eth-bridge-validator/test/mocks"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
)

var (
	db     *Database
	dbConn *gorm.DB
)

func setup() {
	mocks.Setup()
	dbConn, _, _ = helper.SetupSqlMock()

	db = &Database{
		connector: mocks.MConnector,
	}
}

func Test_NewDatabase(t *testing.T) {
	setup()

	actual := NewDatabase(mocks.MConnector)
	assert.NotNil(t, actual)
	assert.Equal(t, db, actual)
}

func Test_GetConnection(t *testing.T) {
	setup()

	mocks.MConnector.On("Connect").Return(dbConn)

	actual := db.Connection()
	assert.Equal(t, dbConn, actual)
}
