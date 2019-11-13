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
	"fmt"
	"net/http"
	"strings"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

const (
	// DO Certificate types
	certTypeLetsEncrypt = "lets_encrypt"
	certTypeCustom      = "custom"

	// Certificate constants
	certPrefix = "do-ccm-"
)

// ensureDomain checks to see if the service contains the annDODomain annotation
// and if it does it verifies the domain exists on the users account
func (l *loadBalancers) ensureDomain(ctx context.Context, service *v1.Service) (*domain, error) {
	domain, err := getDomain(service)
	if err != nil {
		return domain, err
	}

	if domain == nil {
		return nil, nil
	}

	klog.V(2).Infof("Looking up root domain specified in service: %s", domain.root)
	_, _, err = l.resources.gclient.Domains.Get(ctx, domain.root)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve root domain %s: %s", domain.root, err)
	}

	return domain, nil
}

// validateCertificateExistence tests to see if the certificate referenced by the ID exists. If it exists
// the certificate is returned.
func (l *loadBalancers) validateCertificateExistence(ctx context.Context, certificateID string) (*godo.Certificate, error) {
	if certificateID == "" {
		return nil, nil
	}

	certificate, resp, err := l.resources.gclient.Certificates.Get(ctx, certificateID)
	if err != nil && resp.StatusCode != http.StatusNotFound {
		return nil, fmt.Errorf("failed to fetch certificate: %s", err)
	}

	return certificate, nil
}

// validateServiceCertificate ensures the certificate specified in the service annotation
// still exists. If it does not, then the annotation is cleared from the service.
func (l *loadBalancers) validateServiceCertificate(ctx context.Context, service *v1.Service) (*godo.Certificate, error) {
	certificateID := getCertificateID(service)
	klog.V(2).Infof("Looking up certificate for service %s/%s by ID %s", service.Namespace, service.Name, certificateID)
	certificate, err := l.validateCertificateExistence(ctx, certificateID)
	if err != nil {
		return nil, err
	}

	if certificate == nil {
		updateServiceAnnotation(service, annDOCertificateID, "")
	}

	return certificate, nil
}

// ensureCertificateForDomain attempts to fetch a valid certificate for the given domain. If it cannot find an existing valid
// certificate, a new certificate is generated for the domain.
func (l *loadBalancers) ensureCertificateForDomain(ctx context.Context, serviceCertificate *godo.Certificate, domain *domain) (*godo.Certificate, error) {
	if serviceCertificate != nil && isValidCertificateForDomain(serviceCertificate, domain) {
		return serviceCertificate, nil
	}

	serviceCertificate, err := l.findCertificateForDomain(ctx, domain)
	if err != nil {
		return nil, err
	}

	if serviceCertificate == nil {
		serviceCertificate, err = l.generateCertificateForDomain(ctx, domain)
		if err != nil {
			return nil, err
		}
	}

	return serviceCertificate, nil
}

// isValidCertificateForDomain verifies that the certificate DNSNames include the given domain
func isValidCertificateForDomain(certificate *godo.Certificate, domain *domain) bool {
	for _, dnsName := range certificate.DNSNames {
		if dnsName == domain.full {
			// we found matching certificate, break out of ensureCertificate
			return true
		}
	}

	return false
}

// findCertificateForDomain fetches all certificates from the client and attempts to locate a certificate that is
// valid for the given domain.
func (l *loadBalancers) findCertificateForDomain(ctx context.Context, domain *domain) (*godo.Certificate, error) {
	certificates, _, err := l.resources.gclient.Certificates.List(ctx, &godo.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to list certificates: %s", err)
	}

	var certificate *godo.Certificate

	for _, c := range certificates {
		if isValidCertificateForDomain(&c, domain) {
			certificate = &c
			break
		}
	}

	return certificate, nil
}

// generateCertificateForDomain creates a new certificate that is valid for the given domain. If the domain includes
// a subdomain, the generated certificate will include DNSNames for both the root domain and the subdomain.
func (l *loadBalancers) generateCertificateForDomain(ctx context.Context, domain *domain) (*godo.Certificate, error) {
	certName := getCertificateName(domain.full)
	dnsNames := []string{domain.root}

	if domain.sub != "" {
		dnsNames = append(dnsNames, domain.full)
	}

	certificateReq := &godo.CertificateRequest{
		Name:     certName,
		DNSNames: dnsNames,
		Type:     certTypeLetsEncrypt,
	}

	klog.V(2).Infof("Generating new certificate for domain: %s", domain.full)
	certificate, _, err := l.resources.gclient.Certificates.Create(ctx, certificateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	return certificate, nil
}

// findARecordForNameAndIP searches the list of domain records for a Type A record with the given name
// and data pointing to the given IP. If the named A record is found but pointing elsewhere, it throws an error.
func findARecordForNameAndIP(records []godo.DomainRecord, name string, ip string) (*godo.DomainRecord, error) {
	var record *godo.DomainRecord

	for _, r := range records {
		if r.Type != "A" || r.Name != name {
			continue
		}

		if r.Data != ip {
			return nil, fmt.Errorf("the A record(%s) is already in use with another IP(%s)", name, r.Data)
		}

		record = &r
		break
	}

	return record, nil
}

// ensureDomainARecords ensures that if the service has a domain annotation,
// the domain has an A record for the full subdomain pointing to the loadbalancer
func (l *loadBalancers) ensureDomainARecords(ctx context.Context, domain *domain, lb *godo.LoadBalancer) error {
	records, _, err := l.resources.gclient.Domains.Records(ctx, domain.root, &godo.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch records for domain(%s): %s", domain.root, err)
	}

	err = l.ensureDomainARecord(ctx, records, domain.root, "@", lb.IP)
	if err != nil {
		return err
	}

	err = l.ensureDomainARecord(ctx, records, domain.root, domain.sub, lb.IP)
	if err != nil {
		return err
	}

	return nil
}

// ensureDomainARecord takes a list of records for a given domain and verifies the requested A record exists. If it does not, it generates
// a new A record for the given domain, name, and IP.
func (l *loadBalancers) ensureDomainARecord(ctx context.Context, records []godo.DomainRecord, domain string, name string, ip string) error {
	record, err := findARecordForNameAndIP(records, name, ip)
	if err != nil {
		return err
	}

	if record == nil {
		_, _, err = l.resources.gclient.Domains.CreateRecord(ctx, domain, &godo.DomainRecordEditRequest{
			Type: "A",
			Name: name,
			Data: ip,
			TTL:  defaultDomainRecordTTL,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// getCertificateName returns a prefixed certificate so we know to cleanup
// certificate when a loadbalancer for the given domain is deleted
func getCertificateName(fullDomain string) string {
	return fmt.Sprintf("%s%s", certPrefix, strings.ReplaceAll(fullDomain, ".", "-"))
}
