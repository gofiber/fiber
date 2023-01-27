package utils

import (
	"encoding/xml"
	"testing"
)

type serversXMLStructure struct {
	XMLName xml.Name             `xml:"servers"`
	Version string               `xml:"version,attr"`
	Servers []serverXMLStructure `xml:"server"`
}

type serverXMLStructure struct {
	XMLName xml.Name `xml:"server"`
	Name    string   `xml:"name"`
}

const xmlString = `<servers version="1"><server><name>fiber one</name></server><server><name>fiber two</name></server></servers>`

func Test_GolangXMLEncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &serversXMLStructure{
			Version: "1",
			Servers: []serverXMLStructure{
				{Name: "fiber one"},
				{Name: "fiber two"},
			},
		}
		xmlEncoder XMLMarshal = xml.Marshal
	)

	raw, err := xmlEncoder(ss)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), xmlString)
}

func Test_DefaultXMLEncoder(t *testing.T) {
	t.Parallel()

	var (
		ss = &serversXMLStructure{
			Version: "1",
			Servers: []serverXMLStructure{
				{Name: "fiber one"},
				{Name: "fiber two"},
			},
		}
		xmlEncoder XMLMarshal = xml.Marshal
	)

	raw, err := xmlEncoder(ss)
	AssertEqual(t, err, nil)

	AssertEqual(t, string(raw), xmlString)
}
