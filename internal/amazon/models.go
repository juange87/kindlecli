package amazon

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// DeviceInfo holds credentials returned by Amazon device registration.
type DeviceInfo struct {
	DevicePrivateKey string `json:"device_private_key" xml:"device_private_key"`
	ADPToken         string `json:"adp_token" xml:"adp_token"`
	DeviceType       string `json:"device_type" xml:"device_type"`
	GivenName        string `json:"given_name" xml:"given_name"`
	Name             string `json:"name" xml:"name"`
	AccountPool      string `json:"account_pool" xml:"account_pool"`
	UserDirectedID   string `json:"user_directed_id" xml:"user_directed_id"`
	UserDeviceName   string `json:"user_device_name" xml:"user_device_name"`
	HomeRegion       string `json:"home_region,omitempty" xml:"home_region"`
}

// DeviceInfoFromXML parses an XML <response> element into a DeviceInfo.
func DeviceInfoFromXML(data []byte) (DeviceInfo, error) {
	var wrapper struct {
		XMLName xml.Name `xml:"response"`
		DeviceInfo
	}
	if err := xml.Unmarshal(data, &wrapper); err != nil {
		return DeviceInfo{}, err
	}
	return wrapper.DeviceInfo, nil
}

// OwnedDevice represents a Kindle device.
type OwnedDevice struct {
	DeviceName         string `json:"deviceName"`
	DeviceSerialNumber string `json:"deviceSerialNumber"`
}

// GetOwnedDevicesResponse is the JSON response from the owned-devices API.
type GetOwnedDevicesResponse struct {
	OwnedDevices []OwnedDevice `json:"ownedDevices"`
	StatusCode   int           `json:"statusCode"`
}

// ParseGetOwnedDevicesResponse parses JSON into a GetOwnedDevicesResponse.
func ParseGetOwnedDevicesResponse(data []byte) (GetOwnedDevicesResponse, error) {
	var resp GetOwnedDevicesResponse
	err := decodeSTKResponse(data, &resp)
	if err == nil && resp.OwnedDevices == nil {
		err = fmt.Errorf("Amazon response is missing ownedDevices")
	}
	return resp, err
}

// GetUploadUrlResponse is the JSON response from the upload-URL API.
type GetUploadUrlResponse struct {
	ExpiryTime int64  `json:"expiryTime"`
	StatusCode int    `json:"statusCode"`
	STKToken   string `json:"stkToken"`
	UploadURL  string `json:"uploadUrl"`
}

// ParseGetUploadUrlResponse parses JSON into a GetUploadUrlResponse.
func ParseGetUploadUrlResponse(data []byte) (GetUploadUrlResponse, error) {
	var resp GetUploadUrlResponse
	err := decodeSTKResponse(data, &resp)
	if err == nil && (resp.UploadURL == "" || resp.STKToken == "") {
		err = fmt.Errorf("Amazon response is missing upload credentials")
	}
	return resp, err
}

// SendToKindleResponse is the JSON response from the send-to-kindle API.
type SendToKindleResponse struct {
	SKU        string `json:"sku"`
	StatusCode int    `json:"statusCode"`
}

// ParseSendToKindleResponse parses JSON into a SendToKindleResponse.
func ParseSendToKindleResponse(data []byte) (SendToKindleResponse, error) {
	var resp SendToKindleResponse
	err := decodeSTKResponse(data, &resp)
	if err == nil && resp.SKU == "" {
		err = fmt.Errorf("Amazon did not confirm document acceptance")
	}
	return resp, err
}

func decodeSTKResponse(data []byte, dst any) error {
	var status struct {
		Code *int `json:"statusCode"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("invalid Amazon response JSON")
	}
	if status.Code == nil {
		return fmt.Errorf("Amazon response is missing statusCode")
	}
	if *status.Code != 0 {
		return fmt.Errorf("Amazon service returned statusCode %d", *status.Code)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("invalid Amazon response fields")
	}
	return nil
}
