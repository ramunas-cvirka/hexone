package protocols

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRepositorySpecYAML(t *testing.T) {
	specPath := filepath.Join("..", "protocols.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}

	sp, err := LoadSpecYAML(data)
	if err != nil {
		t.Fatalf("LoadSpecYAML: %v", err)
	}

	if _, ok := sp.ProtocolByName("gt06"); !ok {
		t.Fatalf("expected gt06 protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_tcp"); !ok {
		t.Fatalf("expected teltonika_tcp protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_imei_tcp"); !ok {
		t.Fatalf("expected teltonika_imei_tcp protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_udp"); !ok {
		t.Fatalf("expected teltonika_udp protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_codec12"); !ok {
		t.Fatalf("expected teltonika_codec12 protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_codec13"); !ok {
		t.Fatalf("expected teltonika_codec13 protocol")
	}
	if _, ok := sp.ProtocolByName("teltonika_codec14"); !ok {
		t.Fatalf("expected teltonika_codec14 protocol")
	}
}

func TestEnterFieldReaderStopsOnMismatch(t *testing.T) {
	const specYAML = `
version: 1
protocols:
  - name: demo
    endian: be
    layout:
      - field:
          name: payload
          type: bytes
          len: 4
      - hook:
          name: enter_field_reader
          args: { field: payload }
          body:
            - field:
                name: first
                type: u8
            - assert:
                expr: "0 == 1"
                message: "payload mismatch"
            - field:
                name: second
                type: u8
`

	sp, err := LoadSpecYAML([]byte(specYAML))
	if err != nil {
		t.Fatalf("LoadSpecYAML: %v", err)
	}

	res, err := sp.Decode("demo", []byte{0xAA, 0xBB, 0xCC, 0xDD}, NewDefaultHookRegistry())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(res.Errors) != 1 || res.Errors[0] != "payload mismatch" {
		t.Fatalf("unexpected errors: %#v", res.Errors)
	}
	if len(res.Spans) != 1 {
		t.Fatalf("unexpected top-level span count: %d", len(res.Spans))
	}

	payload := res.Spans[0]
	if payload.Name != "payload" {
		t.Fatalf("unexpected top-level span: %q", payload.Name)
	}
	if len(payload.Children) != 2 {
		t.Fatalf("expected first field and one error marker, got %d children", len(payload.Children))
	}

	var first, fail *Span
	for _, child := range payload.Children {
		switch child.Name {
		case "first":
			first = child
		case "assert":
			fail = child
		}
	}
	if first == nil {
		t.Fatalf("missing first child: %+v", payload.Children)
	}
	if first.Start != 0 || first.End != 1 {
		t.Fatalf("unexpected first child: %+v", *first)
	}
	if fail == nil {
		t.Fatalf("missing failure child: %+v", payload.Children)
	}
	if !fail.IsError {
		t.Fatalf("expected error child")
	}
	if fail.Start != 1 || fail.End != 4 {
		t.Fatalf("expected failure span to cover unread payload, got [%d..%d)", fail.Start, fail.End)
	}
}

func TestRepositoryGT06JimiLocationPacket(t *testing.T) {
	specPath := filepath.Join("..", "protocols.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}

	sp, err := LoadSpecYAML(data)
	if err != nil {
		t.Fatalf("LoadSpecYAML: %v", err)
	}

	raw, err := hex.DecodeString("78781F120B081D112E10CC027AC7EB0C46584900148F01CC00287D001FB8000380810D0A")
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}

	res, err := sp.Decode("gt06", raw, NewDefaultHookRegistry())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected decode errors: %#v", res.Errors)
	}

	payload := findSpanNamed(res.Spans, "payload")
	if payload == nil {
		t.Fatalf("missing payload span")
	}
	if payload.Start != 4 || payload.End != 30 {
		t.Fatalf("unexpected payload range: [%d..%d)", payload.Start, payload.End)
	}

	dt := findSpanNamed(payload.Children, "loc.datetime")
	if dt == nil || dt.Value != "2011-08-29 17:46:16" {
		t.Fatalf("unexpected loc.datetime span: %+v", dt)
	}

	lat := findSpanNamed(payload.Children, "loc.lat")
	if lat == nil || lat.Value != "23.111668" {
		t.Fatalf("unexpected loc.lat span: %+v", lat)
	}

	lon := findSpanNamed(payload.Children, "loc.lon")
	if lon == nil || lon.Value != "114.409285" {
		t.Fatalf("unexpected loc.lon span: %+v", lon)
	}

	cellID := findSpanNamed(payload.Children, "loc.cell_id")
	if cellID == nil {
		t.Fatalf("missing loc.cell_id span")
	}
	if cellID.Value != "0x1FB8" {
		t.Fatalf("unexpected loc.cell_id value: %q", cellID.Value)
	}

	serial := findSpanNamed(res.Spans, "serial")
	if serial == nil || serial.Value != "3" {
		t.Fatalf("unexpected serial span: %+v", serial)
	}

	crc := findSpanNamed(res.Spans, "crc")
	if crc == nil || crc.Value != "0x8081" {
		t.Fatalf("unexpected crc span: %+v", crc)
	}
}

func TestRepositoryTeltonikaIMEITCPPacket(t *testing.T) {
	sp := mustRepositorySpec(t)

	raw, err := hex.DecodeString("000F333536333037303432343431303133")
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}

	res, err := sp.Decode("teltonika", raw, NewDefaultHookRegistry())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected decode errors: %#v", res.Errors)
	}

	imei := findSpanNamed(res.Spans, "imei")
	if imei == nil || imei.Value != "356307042441013" {
		t.Fatalf("unexpected imei span: %+v", imei)
	}
}

func TestRepositoryTeltonikaCodec12CommandPacket(t *testing.T) {
	sp := mustRepositorySpec(t)

	raw, err := hex.DecodeString("000000000000000F0C010500000007676574696E666F0100004312")
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}

	res, err := sp.Decode("teltonika", raw, NewDefaultHookRegistry())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected decode errors: %#v", res.Errors)
	}

	body := findSpanNamed(res.Spans, "data_field")
	if body == nil {
		t.Fatalf("missing data_field span")
	}
	msg := findSpanNamed(body.Children, "message")
	if msg == nil || msg.Value != "getinfo" {
		t.Fatalf("unexpected message span: %+v", msg)
	}
}

func mustRepositorySpec(t *testing.T) *Spec {
	t.Helper()

	specPath := filepath.Join("..", "protocols.yaml")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}

	sp, err := LoadSpecYAML(data)
	if err != nil {
		t.Fatalf("LoadSpecYAML: %v", err)
	}
	return sp
}

func findSpanNamed(spans []*Span, name string) *Span {
	for _, sp := range spans {
		if sp == nil {
			continue
		}
		if sp.Name == name {
			return sp
		}
	}
	return nil
}
