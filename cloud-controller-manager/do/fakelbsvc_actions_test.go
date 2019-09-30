/*
Copyright 2017 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import (
	"errors"
	"fmt"
)

type methodKind string

const (
	methodKindGet    methodKind = "Get"
	methodKindList   methodKind = "List"
	methodKindCreate methodKind = "Create"
	methodKindUpdate methodKind = "Update"
	methodKindDelete methodKind = "Delete"
)

type fakeAction interface {
	react(methodKind) (handled bool, result interface{}, err error)
}

type failureAction struct {
	failOnReq         int
	failOnMethodKinds []methodKind
	generateError     func(mk methodKind) error
}

func newUnexpectedCallFailureAction(methodKinds ...methodKind) *failureAction {
	return &failureAction{
		failOnReq:         0,
		failOnMethodKinds: methodKinds,
		generateError: func(mk methodKind) error {
			return fmt.Errorf("method %q should not have been invoked", mk)
		},
	}
}

func newFailureAction(failOnReq int, failErr error, methodKinds ...methodKind) *failureAction {
	if failErr == nil {
		failErr = errors.New("fake service is failing")
	}

	return &failureAction{
		failOnReq:         failOnReq,
		failOnMethodKinds: methodKinds,
		generateError:     func(methodKind) error { return failErr },
	}
}

func (fa *failureAction) react(mk methodKind) (handled bool, result interface{}, err error) {
	if fa.failOnReq <= 0 && isMethodKind(fa.failOnMethodKinds, mk) {
		return true, nil, fa.generateError(mk)
	}

	fa.failOnReq--
	return false, nil, nil
}

func isMethodKind(available []methodKind, mk methodKind) bool {
	if len(available) == 0 {
		return true
	}

	for _, avail := range available {
		if mk == avail {
			return true
		}
	}

	return false
}
