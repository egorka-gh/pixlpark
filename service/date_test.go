package service

import (
	"testing"
)

func TestDateUnmarshal(t *testing.T) {
	dateStr1 := "\"/Date(1331083326130)/\""
	dateStr2 := "\"\\/Date(1564133280000)\\/\""
	date := &Date{}
	//err := json.NewDecoder(strings.NewReader(dateStr)).Decode(&date)
	err := date.UnmarshalJSON([]byte(dateStr1))
	if err != nil {
		t.Errorf("Error parsing Date %q", err.Error())
	}

	if date.format("2006-01-02") != "2012-03-07" {
		t.Errorf("Error parsing Date. Expect %s, got %s", "2012-03-07", date.format("2006-01-02"))
	}

	err = date.UnmarshalJSON([]byte(dateStr2))
	if err != nil {
		t.Errorf("Error parsing Date %q", err.Error())
	}
	if date.format("2006-01-02") != "2019-07-26" {
		t.Errorf("Error parsing Date. Expect %s, got %s", "2019-07-26", date.format("2006-01-02"))
	}

	//dateJ := "{/Date(1564133280000)/}"

}
