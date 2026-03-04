package amazon

import (
	"testing"
)

func TestDeviceInfoFromXML(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<response>
	<device_private_key>-----BEGIN RSA PRIVATE KEY-----
MIIBogIBAAJBALRiMLAA
-----END RSA PRIVATE KEY-----</device_private_key>
	<adp_token>token123</adp_token>
	<device_type>A1K6D1WRW0MALS</device_type>
	<given_name>Juan</given_name>
	<name>Juan Garcia</name>
	<account_pool>Amazon</account_pool>
	<user_directed_id>uid123</user_directed_id>
	<user_device_name>Kindlecli</user_device_name>
	<home_region>NA</home_region>
</response>`)

	info, err := DeviceInfoFromXML(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ADPToken != "token123" {
		t.Errorf("got adp_token=%q, want %q", info.ADPToken, "token123")
	}
	if info.GivenName != "Juan" {
		t.Errorf("got given_name=%q, want %q", info.GivenName, "Juan")
	}
	if info.HomeRegion != "NA" {
		t.Errorf("got home_region=%q, want %q", info.HomeRegion, "NA")
	}
}

func TestGetOwnedDevicesResponseFromJSON(t *testing.T) {
	data := []byte(`{
		"ownedDevices": [
			{
				"deviceCapabilities": {"supportedFormats": true},
				"deviceName": "Kindle Paperwhite",
				"deviceSerialNumber": "G000XX123456"
			}
		],
		"statusCode": 0
	}`)

	resp, err := ParseGetOwnedDevicesResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.OwnedDevices) != 1 {
		t.Fatalf("got %d devices, want 1", len(resp.OwnedDevices))
	}
	if resp.OwnedDevices[0].DeviceSerialNumber != "G000XX123456" {
		t.Errorf("got serial=%q, want %q", resp.OwnedDevices[0].DeviceSerialNumber, "G000XX123456")
	}
}

func TestGetUploadUrlResponseFromJSON(t *testing.T) {
	data := []byte(`{
		"expiryTime": 1234567890,
		"statusCode": 0,
		"stkToken": "stk-abc123",
		"uploadUrl": "https://s3.amazonaws.com/upload/path"
	}`)

	resp, err := ParseGetUploadUrlResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.STKToken != "stk-abc123" {
		t.Errorf("got stkToken=%q, want %q", resp.STKToken, "stk-abc123")
	}
	if resp.UploadURL != "https://s3.amazonaws.com/upload/path" {
		t.Errorf("got uploadUrl=%q, want %q", resp.UploadURL, "https://s3.amazonaws.com/upload/path")
	}
}

func TestSendToKindleResponseFromJSON(t *testing.T) {
	data := []byte(`{"sku": "sku123", "statusCode": 0}`)

	resp, err := ParseSendToKindleResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SKU != "sku123" {
		t.Errorf("got sku=%q, want %q", resp.SKU, "sku123")
	}
}
