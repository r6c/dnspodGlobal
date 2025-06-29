package dnspod

import (
	"context"
	"fmt"

	"github.com/libdns/libdns"
)

// Provider implements the libdns interfaces for DNSPod
type Provider struct {
	// LoginToken is the DNSPod API login token in format "id,token"
	// See https://docs.dnspod.com/api/common-request-parameters/
	LoginToken string `json:"login_token"`

	client *Client
}

// getClient returns an initialized client, creating one if needed
func (p *Provider) getClient() *Client {
	if p.client == nil {
		p.client = newClient(p.LoginToken)
	}
	return p.client
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	client := p.getClient()

	// Get domain ID
	domainID, err := client.getDomainID(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain ID for zone %s: %w", zone, err)
	}

	// List records
	records, err := client.listRecords(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to list records for zone %s: %w", zone, err)
	}

	// Convert to libdns format
	var libRecords []libdns.Record
	for _, rec := range records {
		libRec := convertToLibDNSRecord(rec, zone)
		libRecords = append(libRecords, libRec)
	}

	return libRecords, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()

	// Get domain ID
	domainID, err := client.getDomainID(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain ID for zone %s: %w", zone, err)
	}

	var appendedRecords []libdns.Record

	for _, libRec := range records {
		// Convert to DNSPod format
		rec := convertFromLibDNSRecord(libRec, zone)

		// Create record
		createdRec, err := client.createRecord(ctx, domainID, rec)
		if err != nil {
			return appendedRecords, fmt.Errorf("failed to create record %s: %w", libRec.RR().Name, err)
		}

		// Convert back to libdns format
		newLibRec := convertToLibDNSRecord(*createdRec, zone)
		appendedRecords = append(appendedRecords, newLibRec)
	}

	return appendedRecords, nil
}

// DeleteRecords deletes the records from the zone.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()

	// Get domain ID
	domainID, err := client.getDomainID(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain ID for zone %s: %w", zone, err)
	}

	// Get existing records to find IDs
	existingRecords, err := client.listRecords(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing records: %w", err)
	}

	var deletedRecords []libdns.Record

	for _, libRec := range records {
		// Find matching record by name, type, and value
		rr := libRec.RR()
		var recordID string

		for _, existingRec := range existingRecords {
			existingLibRec := convertToLibDNSRecord(existingRec, zone)
			existingRR := existingLibRec.RR()

			if existingRR.Name == rr.Name &&
				existingRR.Type == rr.Type &&
				existingRR.Data == rr.Data {
				recordID = existingRec.ID
				break
			}
		}

		if recordID == "" {
			return deletedRecords, fmt.Errorf("record not found: %s %s %s", rr.Name, rr.Type, rr.Data)
		}

		// Delete record
		err := client.deleteRecord(ctx, domainID, recordID)
		if err != nil {
			return deletedRecords, fmt.Errorf("failed to delete record %s: %w", rr.Name, err)
		}

		deletedRecords = append(deletedRecords, libRec)
	}

	return deletedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing records
// or creating new ones. It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	client := p.getClient()

	// Get domain ID
	domainID, err := client.getDomainID(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain ID for zone %s: %w", zone, err)
	}

	// Get existing records to find IDs for updates
	existingRecords, err := client.listRecords(ctx, domainID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing records: %w", err)
	}

	var setRecords []libdns.Record

	for _, libRec := range records {
		rr := libRec.RR()
		var recordID string

		// Check if record exists (match by name and type)
		for _, existingRec := range existingRecords {
			existingLibRec := convertToLibDNSRecord(existingRec, zone)
			existingRR := existingLibRec.RR()

			if existingRR.Name == rr.Name && existingRR.Type == rr.Type {
				recordID = existingRec.ID
				break
			}
		}

		rec := convertFromLibDNSRecord(libRec, zone)

		if recordID != "" {
			// Update existing record
			updatedRec, err := client.updateRecord(ctx, domainID, recordID, rec)
			if err != nil {
				return setRecords, fmt.Errorf("failed to update record %s: %w", rr.Name, err)
			}

			newLibRec := convertToLibDNSRecord(*updatedRec, zone)
			setRecords = append(setRecords, newLibRec)
		} else {
			// Create new record
			createdRec, err := client.createRecord(ctx, domainID, rec)
			if err != nil {
				return setRecords, fmt.Errorf("failed to create record %s: %w", rr.Name, err)
			}

			newLibRec := convertToLibDNSRecord(*createdRec, zone)
			setRecords = append(setRecords, newLibRec)
		}
	}

	return setRecords, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
