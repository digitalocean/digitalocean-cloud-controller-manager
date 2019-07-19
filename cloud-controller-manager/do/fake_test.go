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

import "errors"

type fakeService struct {
	// failOnRequest indicates that the fake should return an explicit error on
	// the n-th request, where n is given as zero-based number. Set to a
	// negative number to never return an explicit error.
	failOnRequest int
	// failError is the error to return when failOnRequest is >= 0. When not
	// given, a default error is returned. Ignored when failOnRequest is < 0.
	failError error
}

func (f *fakeService) shouldFail() bool {
	defer func() {
		f.failOnRequest--
	}()
	return f.failOnRequest == 0
}

func newFakeService(failOnReq int, failErr error) *fakeService {
	if failOnReq >= 0 && failErr == nil {
		failErr = errors.New("fake service is failing")
	}

	return &fakeService{
		failOnRequest: failOnReq,
		failError:     failErr,
	}
}
