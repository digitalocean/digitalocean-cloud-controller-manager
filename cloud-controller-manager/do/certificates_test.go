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
	"context"

	"github.com/digitalocean/godo"
)

type fakeCertService struct {
	getFn    func(context.Context, string) (*godo.Certificate, *godo.Response, error)
	listFn   func(context.Context, *godo.ListOptions) ([]godo.Certificate, *godo.Response, error)
	createFn func(context.Context, *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error)
	deleteFn func(ctx context.Context, lbID string) (*godo.Response, error)
}

func (f *fakeCertService) Get(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
	return f.getFn(ctx, certID)
}

func (f *fakeCertService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func (f *fakeCertService) Create(ctx context.Context, crtr *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error) {
	return f.createFn(ctx, crtr)
}

func (f *fakeCertService) Delete(ctx context.Context, certID string) (*godo.Response, error) {
	return f.deleteFn(ctx, certID)
}
