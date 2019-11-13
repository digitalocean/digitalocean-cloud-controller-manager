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
	"errors"
	"reflect"
	"testing"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeDomainService struct {
	domainStore map[string]*godo.Domain
	recordStore map[string]map[int]*godo.DomainRecord

	listFunc   func(context.Context, *godo.ListOptions) ([]godo.Domain, *godo.Response, error)
	getFunc    func(context.Context, string) (*godo.Domain, *godo.Response, error)
	createFunc func(context.Context, *godo.DomainCreateRequest) (*godo.Domain, *godo.Response, error)
	deleteFunc func(context.Context, string) (*godo.Response, error)

	recordsFunc      func(context.Context, string, *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error)
	recordFunc       func(context.Context, string, int) (*godo.DomainRecord, *godo.Response, error)
	deleteRecordFunc func(context.Context, string, int) (*godo.Response, error)
	editRecordFunc   func(context.Context, string, int, *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error)
	createRecordFunc func(context.Context, string, *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error)
}

func (f *fakeDomainService) List(ctx context.Context, opts *godo.ListOptions) ([]godo.Domain, *godo.Response, error) {
	return f.listFunc(ctx, opts)
}

func (f *fakeDomainService) Get(ctx context.Context, domain string) (*godo.Domain, *godo.Response, error) {
	return f.getFunc(ctx, domain)
}

func (f *fakeDomainService) Create(ctx context.Context, opts *godo.DomainCreateRequest) (*godo.Domain, *godo.Response, error) {
	return f.createFunc(ctx, opts)
}

func (f *fakeDomainService) Delete(ctx context.Context, domain string) (*godo.Response, error) {
	return f.deleteFunc(ctx, domain)
}

func (f *fakeDomainService) Records(ctx context.Context, domain string, opts *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
	return f.recordsFunc(ctx, domain, opts)
}

func (f *fakeDomainService) Record(ctx context.Context, domain string, id int) (*godo.DomainRecord, *godo.Response, error) {
	return f.recordFunc(ctx, domain, id)
}

func (f *fakeDomainService) DeleteRecord(ctx context.Context, domain string, id int) (*godo.Response, error) {
	return f.deleteRecordFunc(ctx, domain, id)
}

func (f *fakeDomainService) EditRecord(ctx context.Context, domain string, id int, opts *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error) {
	return f.editRecordFunc(ctx, domain, id, opts)
}

func (f *fakeDomainService) CreateRecord(ctx context.Context, domain string, opts *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error) {
	return f.createRecordFunc(ctx, domain, opts)
}

func newEmptyKVDomainService() fakeDomainService {
	return newKVDomainService(make(map[string]*godo.Domain), make(map[string]map[int]*godo.DomainRecord))
}

func newKVDomainService(domainStore map[string]*godo.Domain, recordStore map[string]map[int]*godo.DomainRecord) fakeDomainService {
	return fakeDomainService{
		domainStore: domainStore,
		recordStore: recordStore,

		createFunc: func(ctx context.Context, opts *godo.DomainCreateRequest) (*godo.Domain, *godo.Response, error) {
			domainStore[opts.Name] = &godo.Domain{
				Name: opts.Name,
			}
			recordStore[opts.Name] = make(map[int]*godo.DomainRecord)

			return domainStore[opts.Name], newFakeOKResponse(), nil
		},
		getFunc: func(ctx context.Context, d string) (*godo.Domain, *godo.Response, error) {
			domain, ok := domainStore[d]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			return domain, newFakeOKResponse(), nil
		},
		createRecordFunc: func(ctx context.Context, domain string, opts *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error) {
			recordMap, ok := recordStore[domain]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			for _, record := range recordMap {
				if record.Name == opts.Name {
					return nil, newFakeBadRequestResponse(), newFakeBadRequestErrorResponse()
				}
			}

			id := len(recordMap) + 1
			recordMap[id] = &godo.DomainRecord{
				ID:       id,
				Type:     opts.Type,
				Name:     opts.Name,
				Data:     opts.Data,
				Priority: opts.Priority,
				Port:     opts.Port,
				TTL:      opts.TTL,
				Weight:   opts.Weight,
				Flags:    opts.Flags,
				Tag:      opts.Tag,
			}

			return recordMap[id], newFakeOKResponse(), nil
		},
		deleteRecordFunc: func(ctx context.Context, domain string, id int) (*godo.Response, error) {
			recordMap, ok := recordStore[domain]
			if !ok {
				return newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			_, ok = recordMap[id]
			if !ok {
				return newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			delete(recordMap, id)
			return newFakeOKResponse(), nil
		},
		recordsFunc: func(ctx context.Context, domain string, opts *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
			recordMap, ok := recordStore[domain]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			records := []godo.DomainRecord{}

			for _, r := range recordMap {
				records = append(records, *r)
			}

			return records, newFakeOKResponse(), nil
		},
		recordFunc: func(ctx context.Context, domain string, id int) (*godo.DomainRecord, *godo.Response, error) {
			recordMap, ok := recordStore[domain]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			record, ok := recordMap[id]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			return record, newFakeOKResponse(), nil
		},
	}
}

func newFakeDomainClient(fakeDomain *fakeDomainService) *godo.Client {
	return newFakeClient(fakeClientOpts{
		fakeDomain: fakeDomain,
	})
}

type fakeCertService struct {
	store      map[string]*godo.Certificate
	getFunc    func(context.Context, string) (*godo.Certificate, *godo.Response, error)
	listFunc   func(context.Context, *godo.ListOptions) ([]godo.Certificate, *godo.Response, error)
	createFunc func(context.Context, *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error)
	deleteFunc func(context.Context, string) (*godo.Response, error)
}

func (f *fakeCertService) Get(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
	return f.getFunc(ctx, certID)
}

func (f *fakeCertService) List(ctx context.Context, opts *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
	return f.listFunc(ctx, opts)
}

func (f *fakeCertService) Create(ctx context.Context, opts *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error) {
	return f.createFunc(ctx, opts)
}

func (f *fakeCertService) Delete(ctx context.Context, certID string) (*godo.Response, error) {
	return f.deleteFunc(ctx, certID)
}

type kvCertService struct {
	reflectorMode bool
	fakeCertService
}

func newKVCertService(store map[string]*godo.Certificate, reflectorMode bool) kvCertService {
	return kvCertService{
		reflectorMode: reflectorMode,
		fakeCertService: fakeCertService{
			store: store,
			getFunc: func(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
				if reflectorMode {
					return &godo.Certificate{
						ID:   certID,
						Type: certTypeLetsEncrypt,
					}, newFakeOKResponse(), nil
				}
				cert, ok := store[certID]
				if ok {
					return cert, newFakeOKResponse(), nil
				}
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			},
			listFunc: func(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
				certificates := []godo.Certificate{}

				for _, v := range store {
					certificates = append(certificates, *v)
				}

				return certificates, newFakeOKResponse(), nil
			},
			createFunc: func(ctx context.Context, crtr *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error) {
				store[crtr.Name] = &godo.Certificate{
					ID:       crtr.Name,
					Name:     crtr.Name,
					Type:     crtr.Type,
					DNSNames: crtr.DNSNames,
				}

				return store[crtr.Name], newFakeOKResponse(), nil
			},
			deleteFunc: func(ctx context.Context, certID string) (*godo.Response, error) {
				if _, ok := store[certID]; !ok {
					return newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
				}

				delete(store, certID)
				return newFakeOKResponse(), nil
			},
		},
	}
}

func createServiceAndCert(lbID, certID, certType string) (*v1.Service, *godo.Certificate) {
	c := &godo.Certificate{
		ID:   certID,
		Type: certType,
	}
	s := createServiceWithCert(lbID, certID)
	return s, c
}

func createServiceWithCert(lbID, certID string) *v1.Service {
	s := createService()
	updateServiceAnnotation(s, annoDOLoadBalancerID, lbID)
	updateServiceAnnotation(s, annDOCertificateID, certID)
	return s
}

func createServiceWithLBID(id string) *v1.Service {
	s := createService()
	updateServiceAnnotation(s, annoDOLoadBalancerID, id)
	return s
}

func createServiceWithDomain(domain string) *v1.Service {
	s := createService()
	updateServiceAnnotation(s, annDODomain, domain)
	return s
}

func createService() *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			UID:  "foobar123",
			Annotations: map[string]string{
				annDOProtocol: "http",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "test",
					Protocol: "TCP",
					Port:     int32(443),
					NodePort: int32(30000),
				},
			},
		},
	}
}

func Test_ensureDomain(t *testing.T) {
	tests := []struct {
		name          string
		domainService func() *fakeDomainService
		service       *v1.Service
		err           error
	}{
		{
			name: "no domain annotation should continue without ensuring",
			domainService: func() *fakeDomainService {
				return &fakeDomainService{
					getFunc: func(ctx context.Context, domain string) (*godo.Domain, *godo.Response, error) {
						return nil, newFakeNotOKResponse(), errors.New("should not fetch domain if not annotation")
					},
				}
			},
			service: createServiceWithDomain(""),
			err:     nil,
		},
		{
			name: "if the root domain does not exist, it should fail to ensure domain",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(make(map[string]*godo.Domain), make(map[string]map[int]*godo.DomainRecord))
				return &domainService
			},
			service: createServiceWithDomain("digitalocean.com"),
			err:     errors.New("failed to retrieve root domain digitalocean.com: FAKE : 404 "),
		},
		{
			name: "if the root domain does exist, it should successfully ensure domain",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, make(map[string]map[int]*godo.DomainRecord))
				return &domainService
			},
			service: createServiceWithDomain("digitalocean.com"),
			err:     nil,
		},
		{
			name: "if passed a subdomain and the root domain does exist, it should successfully ensure domain",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, make(map[string]map[int]*godo.DomainRecord))
				return &domainService
			},
			service: createServiceWithDomain("subdomain.digitalocean.com"),
			err:     nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeClient(fakeClientOpts{
				fakeDomain: test.domainService(),
			})
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			_, err := lb.ensureDomain(context.TODO(), test.service)

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
	}
}

func Test_ensureCertificateForDomain(t *testing.T) {
	tests := []struct {
		name          string
		certService   func() *kvCertService
		serviceCert   *godo.Certificate
		serviceDomain *domain
		expectedCert  *godo.Certificate
		err           error
	}{
		{
			name: "existing certificate valid for domain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{}, false)
				return &certService
			},
			serviceCert: &godo.Certificate{
				DNSNames: []string{"sample.digitalocean.com"},
			},
			serviceDomain: &domain{
				full: "sample.digitalocean.com",
			},
			err: nil,
			expectedCert: &godo.Certificate{
				DNSNames: []string{"sample.digitalocean.com"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeClient(fakeClientOpts{
				fakeCert: &test.certService().fakeCertService,
			})
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			certificate, err := lb.ensureCertificateForDomain(context.TODO(), test.serviceCert, test.serviceDomain)

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if !reflect.DeepEqual(certificate, test.expectedCert) {
				t.Error("certificate does not match")
				t.Logf("expected: %v", test.expectedCert)
				t.Logf("actual: %v", certificate)
			}
		})
	}
}

func Test_isValidCertificateForDomain(t *testing.T) {
	tests := []struct {
		name    string
		cert    *godo.Certificate
		domain  *domain
		isValid bool
	}{
		{
			name: "valid root domain with single dns entry in cert",
			cert: &godo.Certificate{
				ID:       "cert",
				Name:     "cert",
				DNSNames: []string{"digitalocean.com"},
			},
			domain: &domain{
				full: "digitalocean.com",
			},
			isValid: true,
		},
		{
			name: "valid root domain with multiple dns entries in cert",
			cert: &godo.Certificate{
				ID:       "cert",
				Name:     "cert",
				DNSNames: []string{"subdomain.digitalocean.com", "digitalocean.com"},
			},
			domain: &domain{
				full: "digitalocean.com",
			},
			isValid: true,
		},
		{
			name: "invalid subdomain with root dns entry in cert ",
			cert: &godo.Certificate{
				ID:       "cert",
				Name:     "cert",
				DNSNames: []string{"digitalocean.com"},
			},
			domain: &domain{
				full: "subdomain.digitalocean.com",
			},
			isValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := isValidCertificateForDomain(test.cert, test.domain)

			if actual != test.isValid {
				t.Errorf("incorrect check of validity for certificate of domain")
				t.Logf("domain %s and certificate dns %v", test.domain.full, test.cert.DNSNames)
				t.Logf("expected: %t, actual: %t", test.isValid, actual)
			}
		})
	}
}

func Test_findCertificateForDomain(t *testing.T) {
	tests := []struct {
		name         string
		certService  func() *kvCertService
		domain       *domain
		expectedCert *godo.Certificate
		err          error
	}{
		{
			name: "no certificate exists for domain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{}, true)
				return &certService
			},
			domain: &domain{
				full: "digitalocean.com",
			},
			expectedCert: nil,
			err:          nil,
		},
		{
			name: "certificate exists for domain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{
					"cert": {
						ID:       "cert",
						Name:     "cert",
						DNSNames: []string{"digitalocean.com"},
					},
				}, true)
				return &certService
			},
			domain: &domain{
				full: "digitalocean.com",
			},
			expectedCert: &godo.Certificate{
				ID:       "cert",
				Name:     "cert",
				DNSNames: []string{"digitalocean.com"},
			},
			err: nil,
		},
		{
			name: "certificate exists for subdomain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{
					"cert": {
						ID:       "cert",
						Name:     "cert",
						DNSNames: []string{"digitalocean.com", "subdomain.digitalocean.com"},
					},
				}, true)
				return &certService
			},
			domain: &domain{
				full: "subdomain.digitalocean.com",
			},
			expectedCert: &godo.Certificate{
				ID:       "cert",
				Name:     "cert",
				DNSNames: []string{"digitalocean.com", "subdomain.digitalocean.com"},
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeClient(fakeClientOpts{
				fakeCert: &test.certService().fakeCertService,
			})
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			certificate, err := lb.findCertificateForDomain(context.TODO(), test.domain)

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if !reflect.DeepEqual(certificate, test.expectedCert) {
				t.Error("certificate does not match")
				t.Logf("expected: %v", test.expectedCert)
				t.Logf("actual: %v", certificate)
			}
		})
	}
}

func Test_generateCertificateForDomain(t *testing.T) {
	tests := []struct {
		name         string
		certService  func() *kvCertService
		domain       *domain
		expectedCert *godo.Certificate
		err          error
	}{
		{
			name: "root domain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{}, false)
				return &certService
			},
			domain: &domain{
				full: "digitalocean.com",
				root: "digitalocean.com",
			},
			expectedCert: &godo.Certificate{
				ID:       "do-ccm-digitalocean-com",
				Name:     "do-ccm-digitalocean-com",
				Type:     certTypeLetsEncrypt,
				DNSNames: []string{"digitalocean.com"},
			},
			err: nil,
		},
		{
			name: "sub domain",
			certService: func() *kvCertService {
				certService := newKVCertService(map[string]*godo.Certificate{}, false)
				return &certService
			},
			domain: &domain{
				full: "subdomain.digitalocean.com",
				root: "digitalocean.com",
				sub:  "subdomain",
			},
			expectedCert: &godo.Certificate{
				ID:       "do-ccm-subdomain-digitalocean-com",
				Name:     "do-ccm-subdomain-digitalocean-com",
				Type:     certTypeLetsEncrypt,
				DNSNames: []string{"digitalocean.com", "subdomain.digitalocean.com"},
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeClient(fakeClientOpts{
				fakeCert: &test.certService().fakeCertService,
			})
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			certificate, err := lb.generateCertificateForDomain(context.TODO(), test.domain)

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if !reflect.DeepEqual(certificate, test.expectedCert) {
				t.Error("certificate does not match")
				t.Logf("expected: %v", test.expectedCert)
				t.Logf("actual: %v", certificate)
			}
		})
	}
}

func Test_findARecordForNameAndIP(t *testing.T) {
	tests := []struct {
		name           string
		recordName     string
		recordIP       string
		records        []godo.DomainRecord
		err            error
		expectedRecord *godo.DomainRecord
	}{
		{
			name:       "record name exists",
			recordName: "@",
			recordIP:   "10.0.0.1",
			records: []godo.DomainRecord{
				{
					Type: "A",
					Name: "@",
					Data: "10.0.0.1",
				},
			},
			err: nil,
			expectedRecord: &godo.DomainRecord{
				Type: "A",
				Name: "@",
				Data: "10.0.0.1",
			},
		},
		{
			name:       "record name exists but is of type TXT",
			recordName: "@",
			recordIP:   "10.0.0.1",
			records: []godo.DomainRecord{
				{
					Type: "TXT",
					Name: "@",
					Data: "10.0.0.1",
				},
			},
			err:            nil,
			expectedRecord: nil,
		},
		{
			name:       "record name exists but is pointing to a different IP",
			recordName: "@",
			recordIP:   "10.0.0.1",
			records: []godo.DomainRecord{
				{
					Type: "A",
					Name: "@",
					Data: "172.0.0.1",
				},
			},
			err:            errors.New("the A record(@) is already in use with another IP(172.0.0.1)"),
			expectedRecord: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := findARecordForNameAndIP(test.records, test.recordName, test.recordIP)
			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if !reflect.DeepEqual(record, test.expectedRecord) {
				t.Error("record does not match")
				t.Logf("expected: %v", test.expectedRecord)
				t.Logf("actual: %v", record)
			}
		})
	}
}

func Test_ensureDomainARecords(t *testing.T) {
	tests := []struct {
		name            string
		domainService   func() *fakeDomainService
		domain          *domain
		loadbalancer    *godo.LoadBalancer
		expectedRecords []godo.DomainRecord
		err             error
	}{
		{
			name: "service with root domain annotation and existing record for LB",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {
						1: {
							Type: "A",
							ID:   1,
							Name: "@",
							TTL:  defaultDomainRecordTTL,
							Data: "10.0.0.1",
						},
					},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
			},
			expectedRecords: []godo.DomainRecord{
				{
					Type: "A",
					ID:   1,
					Name: "@",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
			},
			err: nil,
		},
		{
			name: "service with root domain annotation and existing record pointing at unexpected LB",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {
						1: {
							Type: "A",
							ID:   1,
							Name: "@",
							TTL:  defaultDomainRecordTTL,
							Data: "172.0.0.1",
						},
					},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
			},
			expectedRecords: []godo.DomainRecord{},
			err:             errors.New("the A record(@) is already in use with another IP(172.0.0.1)"),
		},
		{
			name: "service with root domain annotation and no existing record",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
			},
			expectedRecords: []godo.DomainRecord{
				{
					Type: "A",
					ID:   1,
					Name: "@",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
			},
			err: nil,
		},
		{
			name: "service with sub domain annotation and existing sub record for LB",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {
						1: {
							Type: "A",
							ID:   1,
							Name: "subdomain",
							TTL:  defaultDomainRecordTTL,
							Data: "10.0.0.1",
						},
					},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
				sub:  "subdomain",
			},
			expectedRecords: []godo.DomainRecord{
				{
					Type: "A",
					ID:   2,
					Name: "@",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
				{
					Type: "A",
					ID:   1,
					Name: "subdomain",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
			},
			err: nil,
		},
		{
			name: "service with sub domain annotation and existing sub record pointing at unexpected LB",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {
						1: {
							Type: "A",
							ID:   1,
							Name: "subdomain",
							TTL:  defaultDomainRecordTTL,
							Data: "172.0.0.1",
						},
					},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
				sub:  "subdomain",
			},
			expectedRecords: []godo.DomainRecord{},
			err:             errors.New("the A record(subdomain) is already in use with another IP(172.0.0.1)"),
		},
		{
			name: "service with sub domain annotation and no existing records",
			domainService: func() *fakeDomainService {
				domainService := newKVDomainService(map[string]*godo.Domain{
					"digitalocean.com": {},
				}, map[string]map[int]*godo.DomainRecord{
					"digitalocean.com": {},
				})
				return &domainService
			},
			loadbalancer: createLBWithIP("10.0.0.1"),
			domain: &domain{
				root: "digitalocean.com",
				sub:  "subdomain",
			},
			expectedRecords: []godo.DomainRecord{
				{
					Type: "A",
					ID:   1,
					Name: "@",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
				{
					Type: "A",
					ID:   2,
					Name: "subdomain",
					TTL:  defaultDomainRecordTTL,
					Data: "10.0.0.1",
				},
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeClient(fakeClientOpts{
				fakeDomain: test.domainService(),
			})
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			err := lb.ensureDomainARecords(context.TODO(), test.domain, test.loadbalancer)

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if len(test.expectedRecords) > 0 {
				for _, expected := range test.expectedRecords {
					actual, _, _ := fakeClient.Domains.Record(context.TODO(), test.domain.root, expected.ID)

					if !reflect.DeepEqual(&expected, actual) {
						t.Error("unexpected error")
						t.Logf("expected: %v", expected)
						t.Logf("expected: %v", actual)
					}
				}
			}
		})
	}
}

func Test_LBaaSCertificateScenarios(t *testing.T) {
	tests := []struct {
		name                  string
		setupFn               func(fakeLBService, kvCertService) *v1.Service
		expectedServiceCertID string
		expectedLBCertID      string
		err                   error
	}{
		// lets_encrypt test cases
		{
			name: "[letsencrypt] LB cert ID and service cert ID match",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				return createServiceWithCert(lb.ID, cert.ID)
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID and service cert ID differ and both certs exist",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeLetsEncrypt)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID exists and service cert ID does not",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				service, _ := createServiceAndCert(lb.ID, "meow", certTypeLetsEncrypt)
				return service
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[lets_encrypt] LB cert ID does not exit and service cert ID does",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, _ := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeLetsEncrypt)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},

		// custom test cases
		{
			name: "[custom] LB cert ID and service cert ID match",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				return createServiceWithCert(lb.ID, cert.ID)
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[custom] LB cert ID and service cert ID differ and both certs exist",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeCustom)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
		{
			name: "[custom] LB cert ID does not exit and service cert ID does",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, _ := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeCustom)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
	}

	nodes := []*v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
			},
		},
	}
	droplets := []godo.Droplet{
		{
			ID:   100,
			Name: "node-1",
		},
		{
			ID:   101,
			Name: "node-2",
		},
		{
			ID:   102,
			Name: "node-3",
		},
	}
	fakeDroplet := fakeDropletService{
		listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
			return droplets, newFakeOKResponse(), nil
		},
	}

	for _, test := range tests {
		test := test
		for _, methodName := range []string{"EnsureLoadBalancer", "UpdateLoadBalancer"} {
			t.Run(test.name+"_"+methodName, func(t *testing.T) {
				lbStore := make(map[string]*godo.LoadBalancer)
				lbService := newKVLBService(lbStore)
				certStore := make(map[string]*godo.Certificate)
				certService := newKVCertService(certStore, false)
				service := test.setupFn(lbService, certService)

				fakeClient := newFakeClient(fakeClientOpts{
					fakeDroplet: &fakeDroplet,
					fakeLB:      &lbService,
					fakeCert:    &certService.fakeCertService,
				})
				fakeResources := newResources("", "", fakeClient)
				fakeResources.kclient = fake.NewSimpleClientset()
				if _, err := fakeResources.kclient.CoreV1().Services(service.Namespace).Create(service); err != nil {
					t.Fatalf("failed to add service to fake client: %s", err)
				}

				lb := &loadBalancers{
					resources:         fakeResources,
					region:            "nyc1",
					lbActiveTimeout:   2,
					lbActiveCheckTick: 1,
				}

				var err error
				switch methodName {
				case "EnsureLoadBalancer":
					_, err = lb.EnsureLoadBalancer(context.Background(), "test", service, nodes)
				case "UpdateLoadBalancer":
					err = lb.UpdateLoadBalancer(context.Background(), "test", service, nodes)
				default:
					t.Fatalf("unsupported loadbalancer method: %s", methodName)
				}

				if !reflect.DeepEqual(err, test.err) {
					t.Errorf("got error %q, want: %q", err, test.err)
				}

				service, err = fakeResources.kclient.CoreV1().Services(service.Namespace).Get(service.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get service from fake client: %s", err)
				}

				serviceCertID := getCertificateID(service)
				if test.expectedServiceCertID != serviceCertID {
					t.Errorf("got service certificate ID: %s, want: %s", test.expectedServiceCertID, serviceCertID)
				}

				godoLoadBalancer, _, err := lbService.Get(context.Background(), getLoadBalancerID(service))
				if err != nil {
					t.Fatalf("failed to get loadbalancer %q from fake client: %s", getLoadBalancerID(service), err)
				}
				lbCertID := getCertificateIDFromLB(godoLoadBalancer)
				if test.expectedLBCertID != lbCertID {
					t.Errorf("got load-balancer certificate ID: %s, want: %s", test.expectedLBCertID, lbCertID)
				}
			})
		}
	}
}
