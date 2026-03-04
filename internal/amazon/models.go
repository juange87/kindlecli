package amazon

import (
	"encoding/json"
	"encoding/xml"
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
	err := json.Unmarshal(data, &resp)
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
	err := json.Unmarshal(data, &resp)
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
	err := json.Unmarshal(data, &resp)
	return resp, err
}
