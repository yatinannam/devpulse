package anomaly

import "encoding/json"

// MarshalJSON keeps findings stable for scripting and machine-readable reports.
func (f Finding) MarshalJSON() ([]byte, error) {
    type alias Finding
    return json.Marshal(alias(f))
}
