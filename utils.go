package main

import (
	"fmt"

	"github.com/joeig/go-powerdns/v3"
)

// findRRSet searches through te list of rrsets for a matching entry
// based on type and name.
func findRRSet(rrsets []powerdns.RRset, rrtype powerdns.RRType, name string) *powerdns.RRset {
	for _, rrset := range rrsets {
		if (rrset.Type != nil && *rrset.Type == powerdns.RRTypeTXT) &&
			(rrset.Name != nil && *rrset.Name == name) {
			return &rrset
		}
	}

	return nil
}

// findRecord locates the record entry with the matching content.
func findRecord(records []powerdns.Record, content string) (int, bool) {
	for indx, record := range records {
		if record.Content != nil && *record.Content == content {
			return indx, true
		}
	}

	return -1, false
}

// quote quotes the provide value
func quote(value string) string {
	return fmt.Sprintf("\"%s\"", value)
}
