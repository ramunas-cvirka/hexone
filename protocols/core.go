// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package protocols

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

//
// -------- Spec types --------
//

type Spec struct {
	Version   int        `yaml:"version"`
	Protocols []Protocol `yaml:"protocols"`
}

type Protocol struct {
	Name   string            `yaml:"name"`
	Desc   string            `yaml:"desc"`
	Endian string            `yaml:"endian"`
	Colors map[string]string `yaml:"colors"`
	Layout []Node            `yaml:"layout"`

	endian binary.ByteOrder
}

type Node struct {
	Field  *FieldNode  `yaml:"field,omitempty"`
	Assert *AssertNode `yaml:"assert,omitempty"`
	Switch *SwitchNode `yaml:"switch,omitempty"`
	Repeat *RepeatNode `yaml:"repeat,omitempty"`
	Hook   *HookNode   `yaml:"hook,omitempty"`
	Align  *AlignNode  `yaml:"align,omitempty"`
	Peek   *PeekNode   `yaml:"peek,omitempty"`
	Choose *ChooseNode `yaml:"choose,omitempty"`
	Set    *SetNode    `yaml:"set,omitempty"`
	Route  *RouteNode  `yaml:"route,omitempty"`
}

type FieldNode struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Len        *int   `yaml:"len,omitempty"`
	LenExpr    string `yaml:"len_expr,omitempty"`
	Const      any    `yaml:"const,omitempty"` // numeric const for numeric fields
	Color      string `yaml:"color,omitempty"`
	Desc       string `yaml:"desc,omitempty"`
	ValueFmt   string `yaml:"value_fmt,omitempty"`
	DecodeHook string `yaml:"decode_hook,omitempty"`
	VerifyHook string `yaml:"verify_hook,omitempty"`

	lenExpr  Expr
	typeInfo TypeInfo
	hasConst bool
	constU64 uint64
	constI64 int64
	constIsU bool
}

type AssertNode struct {
	Expr    string `yaml:"expr"`
	Message string `yaml:"message"`

	expr Expr
}

type SwitchNode struct {
	Expr    string            `yaml:"expr"`
	Cases   map[string][]Node `yaml:"cases"`
	Default []Node            `yaml:"default"`

	exprC  Expr
	casesC map[uint64][]Node
}

type RepeatNode struct {
	CountExpr string `yaml:"count_expr"`
	Body      []Node `yaml:"body"`

	countExpr Expr
}

type HookNode struct {
	Name string         `yaml:"name"`
	Args map[string]any `yaml:"args,omitempty"`
	Body []Node         `yaml:"body,omitempty"`
}

type AlignNode struct {
	To    int    `yaml:"to"`
	Fill  string `yaml:"fill,omitempty"` // only "skip" supported
	Color string `yaml:"color,omitempty"`
	Desc  string `yaml:"desc,omitempty"`
}

type PeekNode struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	At      int    `yaml:"at"`
	Len     *int   `yaml:"len,omitempty"`
	LenExpr string `yaml:"len_expr,omitempty"`
	Endian  string `yaml:"endian,omitempty"` // optional override: be/le
	Check   string `yaml:"check,omitempty"`  // optional: ascii_digits
	When    string `yaml:"when,omitempty"`   // optional condition
	Default any    `yaml:"default,omitempty"`

	lenExpr           Expr
	whenExpr          Expr
	typeInfo          TypeInfo
	hasDefault        bool
	defaultValue      Value
	endianOverride    binary.ByteOrder
	hasEndianOverride bool
}

type ChooseNode struct {
	Branches []ChooseBranch `yaml:"branches"`
	Default  []Node         `yaml:"default,omitempty"`
}

type ChooseBranch struct {
	When string `yaml:"when"`
	Body []Node `yaml:"body"`

	whenExpr Expr
}

type SetNode struct {
	Name string `yaml:"name"`
	Expr string `yaml:"expr"`

	exprC Expr
}

type RouteNode struct {
	Peek     []PeekNode        `yaml:"peek,omitempty"`
	Branches []RouteBranch     `yaml:"branches"`
	Targets  map[string][]Node `yaml:"targets"`
	Default  []Node            `yaml:"default,omitempty"`

	targetsC map[string][]Node
}

type RouteBranch struct {
	When string `yaml:"when"`
	To   string `yaml:"to"`

	whenExpr Expr
}

//
// -------- Output types --------
//

type Span struct {
	Name     string
	Desc     string
	Value    string
	ColorKey string

	Start int // inclusive absolute offset
	End   int // exclusive absolute offset

	Children []*Span
	IsError  bool
}

type Result struct {
	Protocol string
	Spans    []*Span
	Errors   []string
}

//
// -------- Public API --------
//

func LoadSpecYAML(yamlBytes []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(yamlBytes))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	for i := range s.Protocols {
		if err := s.Protocols[i].compile(); err != nil {
			return nil, fmt.Errorf("protocol %q: %w", s.Protocols[i].Name, err)
		}
	}
	return &s, nil
}

func (s *Spec) ProtocolByName(name string) (*Protocol, bool) {
	for i := range s.Protocols {
		if s.Protocols[i].Name == name {
			return &s.Protocols[i], true
		}
	}
	return nil, false
}

func (s *Spec) Decode(protocolName string, input []byte, hooks HookRegistry) (Result, error) {
	p, ok := s.ProtocolByName(protocolName)
	if !ok {
		return Result{}, fmt.Errorf("unknown protocol: %s", protocolName)
	}
	return DecodeProtocol(*p, input, hooks)
}

func DecodeProtocol(p Protocol, input []byte, hooks HookRegistry) (Result, error) {
	if hooks == nil {
		hooks = NewDefaultHookRegistry()
	}
	if lf, ok := hooks.(FrameLifecycle); ok {
		lf.BeginFrame(input)
		defer lf.EndFrame()
	}

	ctx := &Ctx{
		Protocol: p.Name,
		Endian:   p.endian,
		Values:   map[string]Value{},
		Hooks:    hooks,
	}

	cur := Cursor{
		Input:  input,
		Base:   0,
		Pos:    0,
		Limit:  len(input),
		Endian: p.endian,
	}

	out := Result{Protocol: p.Name}
	spans, errs := execNodes(ctx, &cur, p.Layout, nil)
	out.Spans = spans
	out.Errors = errs
	return out, nil
}

//
// -------- Compilation --------
//

func (p *Protocol) compile() error {
	switch strings.ToLower(strings.TrimSpace(p.Endian)) {
	case "", "be", "big", "bigendian":
		p.endian = binary.BigEndian
	case "le", "little", "littleendian":
		p.endian = binary.LittleEndian
	default:
		return fmt.Errorf("invalid endian %q", p.Endian)
	}

	for i := range p.Layout {
		if err := compileNode(&p.Layout[i]); err != nil {
			return err
		}
	}
	return nil
}

func compileNode(n *Node) error {
	switch {
	case n.Field != nil:
		return n.Field.compile()
	case n.Assert != nil:
		return n.Assert.compile()
	case n.Switch != nil:
		return n.Switch.compile()
	case n.Repeat != nil:
		return n.Repeat.compile()
	case n.Peek != nil:
		return n.Peek.compile()
	case n.Choose != nil:
		return n.Choose.compile()
	case n.Set != nil:
		return n.Set.compile()
	case n.Route != nil:
		return n.Route.compile()
	case n.Align != nil:
		if n.Align.To <= 0 {
			return fmt.Errorf("align.to must be > 0")
		}
		if n.Align.Fill == "" {
			n.Align.Fill = "skip"
		}
		if n.Align.Fill != "skip" {
			return fmt.Errorf("align.fill only supports 'skip'")
		}
		return nil
	case n.Hook != nil:
		for i := range n.Hook.Body {
			if err := compileNode(&n.Hook.Body[i]); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("empty node")
	}
}

func (f *FieldNode) compile() error {
	ti, ok := typeInfos[f.Type]
	if !ok {
		return fmt.Errorf("unknown field type %q", f.Type)
	}
	f.typeInfo = ti
	f.ValueFmt = strings.ToLower(strings.TrimSpace(f.ValueFmt))

	if ti.Kind == KindBytes {
		if f.Len == nil && strings.TrimSpace(f.LenExpr) == "" {
			return fmt.Errorf("bytes field %q requires len or len_expr", f.Name)
		}
		if f.Len != nil && *f.Len < 0 {
			return fmt.Errorf("bytes field %q len must be >=0", f.Name)
		}
		if strings.TrimSpace(f.LenExpr) != "" {
			ex, err := ParseExpr(f.LenExpr)
			if err != nil {
				return fmt.Errorf("field %q len_expr: %w", f.Name, err)
			}
			f.lenExpr = ex
		}
	} else {
		if f.Len != nil || strings.TrimSpace(f.LenExpr) != "" {
			return fmt.Errorf("numeric field %q must not set len/len_expr", f.Name)
		}
	}

	if f.Const != nil {
		if ti.Kind == KindBytes {
			return fmt.Errorf("field %q: const only valid for numeric types", f.Name)
		}
		u, i, isU, err := parseIntAny(f.Const)
		if err != nil {
			return fmt.Errorf("field %q const: %w", f.Name, err)
		}
		f.hasConst = true
		f.constIsU = isU
		f.constU64 = u
		f.constI64 = i
	}

	return nil
}

func (a *AssertNode) compile() error {
	ex, err := ParseExpr(a.Expr)
	if err != nil {
		return fmt.Errorf("assert expr: %w", err)
	}
	a.expr = ex
	return nil
}

func (r *RepeatNode) compile() error {
	ex, err := ParseExpr(r.CountExpr)
	if err != nil {
		return fmt.Errorf("repeat count_expr: %w", err)
	}
	r.countExpr = ex
	for i := range r.Body {
		if err := compileNode(&r.Body[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *SwitchNode) compile() error {
	ex, err := ParseExpr(s.Expr)
	if err != nil {
		return fmt.Errorf("switch expr: %w", err)
	}
	s.exprC = ex
	s.casesC = make(map[uint64][]Node, len(s.Cases))
	for k, body := range s.Cases {
		u, _, _, err := parseIntString(k)
		if err != nil {
			return fmt.Errorf("switch case key %q: %w", k, err)
		}
		for i := range body {
			if err := compileNode(&body[i]); err != nil {
				return err
			}
		}
		s.casesC[u] = body
	}
	for i := range s.Default {
		if err := compileNode(&s.Default[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *PeekNode) compile() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("peek.name is required")
	}
	ti, ok := typeInfos[p.Type]
	if !ok {
		return fmt.Errorf("peek %q: unknown type %q", p.Name, p.Type)
	}
	p.typeInfo = ti

	if strings.TrimSpace(p.When) != "" {
		ex, err := ParseExpr(p.When)
		if err != nil {
			return fmt.Errorf("peek %q when: %w", p.Name, err)
		}
		p.whenExpr = ex
	}

	switch strings.ToLower(strings.TrimSpace(p.Endian)) {
	case "":
		// Use protocol/context endian.
	case "be", "big", "bigendian":
		p.endianOverride = binary.BigEndian
		p.hasEndianOverride = true
	case "le", "little", "littleendian":
		p.endianOverride = binary.LittleEndian
		p.hasEndianOverride = true
	default:
		return fmt.Errorf("peek %q: invalid endian %q", p.Name, p.Endian)
	}

	p.Check = strings.ToLower(strings.TrimSpace(p.Check))

	if ti.Kind == KindBytes {
		if p.Len == nil && strings.TrimSpace(p.LenExpr) == "" {
			return fmt.Errorf("peek %q bytes requires len or len_expr", p.Name)
		}
		if p.Len != nil && *p.Len < 0 {
			return fmt.Errorf("peek %q len must be >=0", p.Name)
		}
		if strings.TrimSpace(p.LenExpr) != "" {
			ex, err := ParseExpr(p.LenExpr)
			if err != nil {
				return fmt.Errorf("peek %q len_expr: %w", p.Name, err)
			}
			p.lenExpr = ex
		}
		if p.Check == "" {
			return fmt.Errorf("peek %q bytes requires check (supported: ascii_digits)", p.Name)
		}
		if p.Check != "ascii_digits" {
			return fmt.Errorf("peek %q: unknown check %q", p.Name, p.Check)
		}
	} else {
		if p.Len != nil || strings.TrimSpace(p.LenExpr) != "" {
			return fmt.Errorf("peek %q numeric must not set len/len_expr", p.Name)
		}
		if p.Check != "" {
			return fmt.Errorf("peek %q numeric must not set check", p.Name)
		}
	}

	if p.Default != nil {
		p.hasDefault = true
		v, err := parsePeekDefault(p)
		if err != nil {
			return err
		}
		p.defaultValue = v
	} else {
		p.defaultValue = inferPeekDefault(p)
	}

	return nil
}

func (c *ChooseNode) compile() error {
	if len(c.Branches) == 0 {
		return fmt.Errorf("choose.branches must not be empty")
	}
	for i := range c.Branches {
		w := strings.TrimSpace(c.Branches[i].When)
		if w == "" {
			return fmt.Errorf("choose branch %d missing when", i)
		}
		ex, err := ParseExpr(w)
		if err != nil {
			return fmt.Errorf("choose branch %d when: %w", i, err)
		}
		c.Branches[i].whenExpr = ex
		for j := range c.Branches[i].Body {
			if err := compileNode(&c.Branches[i].Body[j]); err != nil {
				return err
			}
		}
	}
	for i := range c.Default {
		if err := compileNode(&c.Default[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *SetNode) compile() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("set.name is required")
	}
	if strings.TrimSpace(s.Expr) == "" {
		return fmt.Errorf("set %q expr is required", s.Name)
	}
	ex, err := ParseExpr(s.Expr)
	if err != nil {
		return fmt.Errorf("set %q expr: %w", s.Name, err)
	}
	s.exprC = ex
	return nil
}

func (r *RouteNode) compile() error {
	if len(r.Branches) == 0 {
		return fmt.Errorf("route.branches must not be empty")
	}

	for i := range r.Peek {
		if err := r.Peek[i].compile(); err != nil {
			return err
		}
	}

	if len(r.Targets) == 0 {
		return fmt.Errorf("route.targets must not be empty")
	}
	r.targetsC = make(map[string][]Node, len(r.Targets))
	for name, body := range r.Targets {
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("route target name must not be empty")
		}
		for i := range body {
			if err := compileNode(&body[i]); err != nil {
				return err
			}
		}
		r.targetsC[name] = body
	}

	for i := range r.Branches {
		w := strings.TrimSpace(r.Branches[i].When)
		if w == "" {
			return fmt.Errorf("route branch %d missing when", i)
		}
		ex, err := ParseExpr(w)
		if err != nil {
			return fmt.Errorf("route branch %d when: %w", i, err)
		}
		r.Branches[i].whenExpr = ex

		to := strings.TrimSpace(r.Branches[i].To)
		if to == "" {
			return fmt.Errorf("route branch %d missing to", i)
		}
		if _, ok := r.targetsC[to]; !ok {
			return fmt.Errorf("route branch %d: unknown target %q", i, to)
		}
	}

	for i := range r.Default {
		if err := compileNode(&r.Default[i]); err != nil {
			return err
		}
	}

	return nil
}

//
// -------- Interpreter core --------
//

type Cursor struct {
	Input  []byte
	Base   int
	Pos    int
	Limit  int
	Endian binary.ByteOrder
}

func (c *Cursor) Offset() int { return c.Base + c.Pos }
func (c *Cursor) Remaining() int {
	if c.Pos >= c.Limit {
		return 0
	}
	return c.Limit - c.Pos
}
func (c *Cursor) readN(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("readN negative: %d", n)
	}
	if c.Pos+n > c.Limit {
		return nil, fmt.Errorf("unexpected EOF at offset %d: need %d bytes, remaining %d", c.Offset(), n, c.Remaining())
	}
	b := c.Input[c.Base+c.Pos : c.Base+c.Pos+n]
	c.Pos += n
	return b, nil
}

type ValueKind int

const (
	VKInt ValueKind = iota
	VKUint
	VKBool
	VKBytesLen
)

type Value struct {
	Kind ValueKind
	I64  int64
	U64  uint64
	Bool bool
}

type Ctx struct {
	Protocol string
	Endian   binary.ByteOrder
	Values   map[string]Value
	Hooks    HookRegistry
}

func execNodes(ctx *Ctx, cur *Cursor, nodes []Node, parent *Span) ([]*Span, []string) {
	return execNodesMode(ctx, cur, nodes, parent, false)
}

func execNodesStrict(ctx *Ctx, cur *Cursor, nodes []Node, parent *Span) ([]*Span, []string) {
	return execNodesMode(ctx, cur, nodes, parent, true)
}

func execNodesMode(ctx *Ctx, cur *Cursor, nodes []Node, parent *Span, stopOnError bool) ([]*Span, []string) {
	var spans []*Span
	var errs []string

	for i := range nodes {
		n := nodes[i]
		var nodeErrs []string
		switch {
		case n.Field != nil:
			sp, es := execField(ctx, cur, n.Field)
			if sp != nil {
				spans = append(spans, sp)
				ctx.Hooks.NoteFieldSpan(sp.Name, sp)
			}
			nodeErrs = es

		case n.Assert != nil:
			nodeErrs = execAssert(ctx, cur, n.Assert, parent)

		case n.Switch != nil:
			ss, es := execSwitch(ctx, cur, n.Switch, parent)
			spans = append(spans, ss...)
			nodeErrs = es

		case n.Repeat != nil:
			rs, es := execRepeat(ctx, cur, n.Repeat, parent)
			spans = append(spans, rs...)
			nodeErrs = es

		case n.Peek != nil:
			nodeErrs = execPeek(ctx, cur, n.Peek)

		case n.Choose != nil:
			cs, es := execChoose(ctx, cur, n.Choose, parent)
			spans = append(spans, cs...)
			nodeErrs = es

		case n.Set != nil:
			nodeErrs = execSet(ctx, cur, n.Set)

		case n.Route != nil:
			rs, es := execRoute(ctx, cur, n.Route, parent)
			spans = append(spans, rs...)
			nodeErrs = es

		case n.Align != nil:
			sp, es := execAlign(ctx, cur, n.Align)
			if sp != nil {
				spans = append(spans, sp)
			}
			nodeErrs = es

		case n.Hook != nil:
			hs, es := execHook(ctx, cur, n.Hook, parent)
			spans = append(spans, hs...)
			nodeErrs = es

		default:
			nodeErrs = []string{"unknown/empty node"}
		}

		if len(nodeErrs) > 0 {
			errs = append(errs, nodeErrs...)
			if stopOnError {
				break
			}
		}
	}

	return spans, errs
}

func execField(ctx *Ctx, cur *Cursor, f *FieldNode) (*Span, []string) {
	var errs []string
	start := cur.Offset()

	switch f.typeInfo.Kind {
	case KindBytes:
		var n int
		if f.Len != nil {
			n = *f.Len
		} else {
			v, err := EvalInt(ctx, cur, f.lenExpr)
			if err != nil {
				errs = append(errs, fmt.Sprintf("field %s len_expr: %v", f.Name, err))
				return &Span{Name: f.Name, Desc: f.Desc, Value: "len_expr error", ColorKey: nz(f.Color, "error"), Start: start, End: start, IsError: true}, errs
			}
			n = v
		}
		b, err := cur.readN(n)
		if err != nil {
			errs = append(errs, fmt.Sprintf("field %s: %v", f.Name, err))
			end := cur.Offset()
			ctx.Values[f.Name] = Value{Kind: VKBytesLen, U64: uint64(max(0, end-start))}
			return &Span{Name: f.Name, Desc: f.Desc, Value: fmt.Sprintf("bytes[%d] EOF", n), ColorKey: nz(f.Color, "error"), Start: start, End: end, IsError: true}, errs
		}
		end := start + len(b)
		ctx.Values[f.Name] = Value{Kind: VKBytesLen, U64: uint64(len(b))}
		valStr := formatBytesValue(f, b)
		return &Span{Name: f.Name, Desc: f.Desc, Value: valStr, ColorKey: nz(f.Color, "payload"), Start: start, End: end}, errs

	case KindNum:
		b, err := cur.readN(f.typeInfo.Size)
		if err != nil {
			errs = append(errs, fmt.Sprintf("field %s: %v", f.Name, err))
			end := cur.Offset()
			return &Span{Name: f.Name, Desc: f.Desc, Value: "EOF", ColorKey: nz(f.Color, "error"), Start: start, End: end, IsError: true}, errs
		}
		end := start + len(b)

		i64, u64 := readNumber(cur.Endian, b, f.typeInfo.Signed)
		var valStr string
		if f.typeInfo.Signed {
			ctx.Values[f.Name] = Value{Kind: VKInt, I64: i64}
		} else {
			ctx.Values[f.Name] = Value{Kind: VKUint, U64: u64}
		}
		valStr = formatNumericValue(f, i64, u64, f.typeInfo.Signed)

		if f.hasConst {
			ok := false
			if f.constIsU {
				if f.typeInfo.Signed {
					ok = uint64(i64) == f.constU64
				} else {
					ok = u64 == f.constU64
				}
			} else {
				if f.typeInfo.Signed {
					ok = i64 == f.constI64
				} else {
					ok = int64(u64) == f.constI64
				}
			}
			if !ok {
				msg := fmt.Sprintf("const mismatch: got %s", valStr)
				errs = append(errs, fmt.Sprintf("field %s: %s", f.Name, msg))
				sp := &Span{Name: f.Name, Desc: f.Desc, Value: valStr, ColorKey: nz(f.Color, "payload"), Start: start, End: end}
				attachErrorChild(sp, msg, start, end)
				return sp, errs
			}
		}

		return &Span{Name: f.Name, Desc: f.Desc, Value: valStr, ColorKey: nz(f.Color, "payload"), Start: start, End: end}, errs

	default:
		return &Span{Name: f.Name, Desc: f.Desc, Value: "unsupported", ColorKey: "error", Start: start, End: start, IsError: true}, []string{"unsupported field kind"}
	}
}

func execAssert(ctx *Ctx, cur *Cursor, a *AssertNode, parent *Span) []string {
	b, err := EvalBool(ctx, cur, a.expr)
	if err != nil {
		return []string{fmt.Sprintf("assert eval error: %v", err)}
	}
	if b {
		return nil
	}
	msg := nz(a.Message, "assert failed")
	start, end := errorSpanRange(cur, parent)
	es := &Span{Name: "assert", Desc: msg, Value: "false", ColorKey: "error", Start: start, End: end, IsError: true}
	if parent != nil {
		parent.Children = append(parent.Children, es)
	}
	return []string{msg}
}

func execSwitch(ctx *Ctx, cur *Cursor, s *SwitchNode, parent *Span) ([]*Span, []string) {
	v, err := EvalUint(ctx, cur, s.exprC)
	if err != nil {
		return nil, []string{fmt.Sprintf("switch expr: %v", err)}
	}
	body, ok := s.casesC[v]
	if !ok {
		body = s.Default
	}
	if len(body) == 0 {
		return nil, nil
	}
	return execNodes(ctx, cur, body, parent)
}

func execRepeat(ctx *Ctx, cur *Cursor, r *RepeatNode, parent *Span) ([]*Span, []string) {
	n, err := EvalInt(ctx, cur, r.countExpr)
	if err != nil {
		return nil, []string{fmt.Sprintf("repeat count_expr: %v", err)}
	}
	if n < 0 {
		return nil, []string{fmt.Sprintf("repeat count negative: %d", n)}
	}
	var spans []*Span
	var errs []string
	for i := 0; i < n; i++ {
		ss, es := execNodes(ctx, cur, r.Body, parent)
		spans = append(spans, ss...)
		errs = append(errs, es...)
		if cur.Remaining() == 0 {
			break
		}
	}
	return spans, errs
}

func execPeek(ctx *Ctx, cur *Cursor, p *PeekNode) []string {
	if p.whenExpr != nil {
		ok, err := EvalBool(ctx, cur, p.whenExpr)
		if err != nil {
			return []string{fmt.Sprintf("peek %s when: %v", p.Name, err)}
		}
		if !ok {
			ctx.Values[p.Name] = p.defaultValue
			return nil
		}
	}

	switch p.typeInfo.Kind {
	case KindNum:
		start := p.At
		end := start + p.typeInfo.Size
		if start < 0 || end < start || end > cur.Limit {
			return []string{
				fmt.Sprintf(
					"peek %s: out of range at=%d size=%d frame_size=%d",
					p.Name, p.At, p.typeInfo.Size, cur.Limit,
				),
			}
		}
		raw := cur.Input[cur.Base+start : cur.Base+end]
		bo := cur.Endian
		if p.hasEndianOverride {
			bo = p.endianOverride
		}
		i64, u64 := readNumber(bo, raw, p.typeInfo.Signed)
		if p.typeInfo.Signed {
			ctx.Values[p.Name] = Value{Kind: VKInt, I64: i64}
		} else {
			ctx.Values[p.Name] = Value{Kind: VKUint, U64: u64}
		}
		return nil

	case KindBytes:
		n := 0
		if p.Len != nil {
			n = *p.Len
		} else {
			v, err := EvalInt(ctx, cur, p.lenExpr)
			if err != nil {
				return []string{fmt.Sprintf("peek %s len_expr: %v", p.Name, err)}
			}
			n = v
		}
		if n < 0 {
			return []string{fmt.Sprintf("peek %s: negative length %d", p.Name, n)}
		}
		start := p.At
		end := start + n
		if start < 0 || end < start || end > cur.Limit {
			return []string{
				fmt.Sprintf(
					"peek %s: out of range at=%d len=%d frame_size=%d",
					p.Name, p.At, n, cur.Limit,
				),
			}
		}
		raw := cur.Input[cur.Base+start : cur.Base+end]
		switch p.Check {
		case "ascii_digits":
			ctx.Values[p.Name] = Value{Kind: VKBool, Bool: isASCIIDigits(raw)}
		default:
			// Should be prevented by compile().
			ctx.Values[p.Name] = Value{Kind: VKBytesLen, U64: uint64(len(raw))}
		}
		return nil
	}

	return []string{fmt.Sprintf("peek %s: unsupported type kind", p.Name)}
}

func execChoose(ctx *Ctx, cur *Cursor, c *ChooseNode, parent *Span) ([]*Span, []string) {
	for _, br := range c.Branches {
		ok, err := EvalBool(ctx, cur, br.whenExpr)
		if err != nil {
			return nil, []string{fmt.Sprintf("choose when: %v", err)}
		}
		if !ok {
			continue
		}
		return execNodes(ctx, cur, br.Body, parent)
	}
	if len(c.Default) == 0 {
		return nil, nil
	}
	return execNodes(ctx, cur, c.Default, parent)
}

func execSet(ctx *Ctx, cur *Cursor, s *SetNode) []string {
	v, err := s.exprC.Eval(ctx, cur)
	if err != nil {
		return []string{fmt.Sprintf("set %s: %v", s.Name, err)}
	}
	ctx.Values[s.Name] = v
	return nil
}

func execRoute(ctx *Ctx, cur *Cursor, r *RouteNode, parent *Span) ([]*Span, []string) {
	var errs []string
	for i := range r.Peek {
		errs = append(errs, execPeek(ctx, cur, &r.Peek[i])...)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	for _, br := range r.Branches {
		ok, err := EvalBool(ctx, cur, br.whenExpr)
		if err != nil {
			return nil, append(errs, fmt.Sprintf("route when: %v", err))
		}
		if !ok {
			continue
		}
		body := r.targetsC[br.To]
		if len(body) == 0 {
			return nil, errs
		}
		ss, es := execNodes(ctx, cur, body, parent)
		return ss, append(errs, es...)
	}

	if len(r.Default) == 0 {
		return nil, errs
	}
	ss, es := execNodes(ctx, cur, r.Default, parent)
	return ss, append(errs, es...)
}

func execAlign(ctx *Ctx, cur *Cursor, a *AlignNode) (*Span, []string) {
	start := cur.Offset()
	if a.To <= 1 {
		return nil, nil
	}
	off := cur.Offset()
	mod := off % a.To
	if mod == 0 {
		return nil, nil
	}
	skip := a.To - mod
	_, err := cur.readN(skip)
	if err != nil {
		return &Span{Name: "align", Desc: nz(a.Desc, fmt.Sprintf("align to %d", a.To)), Value: fmt.Sprintf("skip %d EOF", skip), ColorKey: nz(a.Color, "meta"), Start: start, End: cur.Offset(), IsError: true}, []string{err.Error()}
	}
	return &Span{Name: "align", Desc: nz(a.Desc, fmt.Sprintf("align to %d", a.To)), Value: fmt.Sprintf("skip %d", skip), ColorKey: nz(a.Color, "meta"), Start: start, End: cur.Offset()}, nil
}

func execHook(ctx *Ctx, cur *Cursor, h *HookNode, parent *Span) ([]*Span, []string) {
	// Built-in: enter_field_reader
	if h.Name == "enter_field_reader" {
		return hookEnterFieldReader(ctx, cur, h, parent)
	}
	if vf := ctx.Hooks.GetVerify(h.Name); vf != nil {
		if err := vf(ctx, VerifyHookInput{Args: h.Args}); err != nil {
			return nil, []string{fmt.Sprintf("hook %s: %v", h.Name, err)}
		}
		if len(h.Body) > 0 {
			ss, es := execNodes(ctx, cur, h.Body, parent)
			return ss, es
		}
		return nil, nil
	}

	fn := ctx.Hooks.GetHook(h.Name)
	if fn == nil {
		return nil, []string{fmt.Sprintf("missing hook: %s", h.Name)}
	}
	spans, err := fn(ctx, cur, h.Args)
	if err != nil {
		return spans, []string{fmt.Sprintf("hook %s: %v", h.Name, err)}
	}
	if len(h.Body) > 0 {
		ss, es := execNodes(ctx, cur, h.Body, parent)
		spans = append(spans, ss...)
		return spans, es
	}
	return spans, nil
}

func hookEnterFieldReader(ctx *Ctx, cur *Cursor, h *HookNode, parent *Span) ([]*Span, []string) {
	raw, ok := h.Args["field"]
	if !ok {
		return nil, []string{"enter_field_reader missing args.field"}
	}
	fieldName, _ := raw.(string)
	if fieldName == "" {
		return nil, []string{"enter_field_reader args.field must be string"}
	}

	base, sl, ok := ctx.Hooks.ResolveFieldSlice(fieldName)
	if !ok {
		return nil, []string{fmt.Sprintf("enter_field_reader: cannot resolve field slice for %q", fieldName)}
	}

	sub := Cursor{
		Input:  cur.Input,
		Base:   base,
		Pos:    0,
		Limit:  len(sl),
		Endian: ctx.Endian,
	}

	owner := ctx.Hooks.ResolveFieldSpan(fieldName)
	if owner == nil {
		owner = parent
	}

	children, errs := execNodesStrict(ctx, &sub, h.Body, owner)
	if owner != nil && len(children) > 0 {
		owner.Children = append(owner.Children, children...)
	}
	if owner != nil && sub.Remaining() > 0 {
		start := sub.Offset()
		end := sub.Base + sub.Limit
		isInvalid := len(errs) > 0
		if isInvalid && spansCoverRange(owner.Children, start, end) {
			return nil, errs
		}
		name := fieldName + ".unparsed"
		desc := "bytes not consumed by sub-decoder"
		colorKey := "meta"
		if isInvalid {
			name = fieldName + ".invalid"
			desc = "decode stopped after structural mismatch"
			colorKey = "error"
		}
		owner.Children = append(owner.Children, &Span{
			Name:     name,
			Desc:     desc,
			Value:    fmt.Sprintf("len=%d", end-start),
			ColorKey: colorKey,
			Start:    start,
			End:      end,
			IsError:  isInvalid,
		})
	}
	return nil, errs
}

func isASCIIDigits(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, c := range b {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func errorSpanRange(cur *Cursor, parent *Span) (int, int) {
	off := cur.Offset()
	if parent == nil {
		return off, off
	}
	if off < parent.End {
		return off, parent.End
	}
	if parent.End > parent.Start {
		return parent.End - 1, parent.End
	}
	return off, off
}

func spansCoverRange(spans []*Span, start, end int) bool {
	for _, sp := range spans {
		if sp == nil || !sp.IsError {
			continue
		}
		if sp.Start <= start && sp.End >= end {
			return true
		}
	}
	return false
}

func attachErrorChild(sp *Span, msg string, start, end int) {
	if sp == nil {
		return
	}
	sp.Children = append(sp.Children, &Span{
		Name:     "error",
		Desc:     msg,
		Value:    "",
		ColorKey: "error",
		Start:    start,
		End:      end,
		IsError:  true,
	})
}

//
// -------- Hooks registry --------
//

type FrameLifecycle interface {
	BeginFrame(input []byte)
	EndFrame()
}

type DecodeHookInput struct {
	FieldName string
	Base      int
	Bytes     []byte
}

type DecodeHook func(ctx *Ctx, in DecodeHookInput) ([]*Span, error)

type VerifyHookInput struct {
	Args map[string]any
}

type VerifyHook func(ctx *Ctx, in VerifyHookInput) error

type Hook func(ctx *Ctx, cur *Cursor, args map[string]any) ([]*Span, error)

type HookRegistry interface {
	GetDecode(name string) DecodeHook
	GetVerify(name string) VerifyHook
	GetHook(name string) Hook

	NoteFieldSpan(fieldName string, sp *Span)
	ResolveFieldSlice(fieldName string) (base int, slice []byte, ok bool)
	ResolveFieldSpan(fieldName string) *Span

	BeginFrame(input []byte)
	EndFrame()
}

type DefaultHookRegistry struct {
	decode map[string]DecodeHook
	verify map[string]VerifyHook
	hooks  map[string]Hook
	input  []byte
	last   map[string]*Span
	slices map[string]struct{ base, end int }
}

func NewDefaultHookRegistry() *DefaultHookRegistry {
	r := &DefaultHookRegistry{
		decode: make(map[string]DecodeHook),
		verify: make(map[string]VerifyHook),
		hooks:  make(map[string]Hook),
		last:   make(map[string]*Span),
		slices: make(map[string]struct{ base, end int }),
	}

	// Built-in verify hooks
	r.verify["verify_gt06_crc16_itu"] = verifyGT06CRC16ITU
	r.verify["verify_teltonika_crc16_ibm"] = verifyTeltonikaCRC16IBM

	// You can register custom hooks later:
	// r.hooks["teltonika_io_rest"] = ...
	// r.hooks["teltonika_codec8e_rest"] = ...

	return r
}

func (r *DefaultHookRegistry) BeginFrame(input []byte) {
	r.input = input
	for k := range r.last {
		delete(r.last, k)
	}
	for k := range r.slices {
		delete(r.slices, k)
	}
}
func (r *DefaultHookRegistry) EndFrame() { r.input = nil }

func (r *DefaultHookRegistry) GetDecode(name string) DecodeHook { return r.decode[name] }
func (r *DefaultHookRegistry) GetVerify(name string) VerifyHook { return r.verify[name] }
func (r *DefaultHookRegistry) GetHook(name string) Hook         { return r.hooks[name] }

func (r *DefaultHookRegistry) NoteFieldSpan(fieldName string, sp *Span) {
	if sp == nil {
		return
	}
	r.last[fieldName] = sp
	r.slices[fieldName] = struct{ base, end int }{base: sp.Start, end: sp.End}
}

func (r *DefaultHookRegistry) ResolveFieldSlice(fieldName string) (base int, slice []byte, ok bool) {
	if r.input == nil {
		return 0, nil, false
	}
	se, ok := r.slices[fieldName]
	if !ok || se.base < 0 || se.end < se.base || se.end > len(r.input) {
		return 0, nil, false
	}
	return se.base, r.input[se.base:se.end], true
}

func (r *DefaultHookRegistry) ResolveFieldSpan(fieldName string) *Span {
	return r.last[fieldName]
}

//
// -------- Built-in verify hooks --------
//

func verifyGT06CRC16ITU(ctx *Ctx, in VerifyHookInput) error {
	// Args: length_field, serial_field, crc_field (names)
	lengthName := argString(in.Args, "length_field")
	serialName := argString(in.Args, "serial_field")
	crcName := argString(in.Args, "crc_field")
	if lengthName == "" || serialName == "" || crcName == "" {
		return errors.New("verify_gt06_crc16_itu requires args: length_field, serial_field, crc_field")
	}

	lengthSp := ctx.Hooks.ResolveFieldSpan(lengthName)
	serialSp := ctx.Hooks.ResolveFieldSpan(serialName)
	crcSp := ctx.Hooks.ResolveFieldSpan(crcName)
	if lengthSp == nil || serialSp == nil || crcSp == nil {
		return fmt.Errorf("missing spans: length=%v serial=%v crc=%v", lengthSp != nil, serialSp != nil, crcSp != nil)
	}
	// CRC input is bytes from LENGTH field start through SERIAL end (inclusive range => [start:end))
	if serialSp.End > len(ctx.Hooks.(*DefaultHookRegistry).input) {
		return errors.New("internal: serial span out of bounds")
	}
	raw := ctx.Hooks.(*DefaultHookRegistry).input
	data := raw[lengthSp.Start:serialSp.End]

	want := uint16FromSpan(raw, crcSp, ctx.Endian)
	got := crc16X25(data)

	if got != want {
		return fmt.Errorf("CRC16-ITU mismatch: calc=0x%04X pkt=0x%04X", got, want)
	}
	return nil
}

func verifyTeltonikaCRC16IBM(ctx *Ctx, in VerifyHookInput) error {
	// Args: data_field, crc_field
	dataName := argString(in.Args, "data_field")
	crcName := argString(in.Args, "crc_field")
	if dataName == "" || crcName == "" {
		return errors.New("verify_teltonika_crc16_ibm requires args: data_field, crc_field")
	}

	reg, ok := ctx.Hooks.(*DefaultHookRegistry)
	if !ok || reg.input == nil {
		return errors.New("internal: default registry required for verify_teltonika_crc16_ibm")
	}
	raw := reg.input

	dataSp := ctx.Hooks.ResolveFieldSpan(dataName)
	crcSp := ctx.Hooks.ResolveFieldSpan(crcName)
	if dataSp == nil || crcSp == nil {
		return fmt.Errorf("missing spans: data_field=%v crc_field=%v", dataSp != nil, crcSp != nil)
	}

	data := raw[dataSp.Start:dataSp.End]
	crc32 := uint32FromSpan(raw, crcSp, ctx.Endian)

	want := uint16(crc32 & 0xFFFF) // low 16 bits hold CRC
	got := crc16IBM(data)          // poly 0xA001, init 0x0000

	if got != want {
		return fmt.Errorf("CRC16/IBM mismatch: calc=0x%04X pkt=0x%04X (crc32=0x%08X)", got, want, crc32)
	}
	return nil
}

//
// -------- Expressions --------
//

type Expr interface {
	Eval(ctx *Ctx, cur *Cursor) (Value, error)
	String() string
}

func EvalInt(ctx *Ctx, cur *Cursor, e Expr) (int, error) {
	v, err := e.Eval(ctx, cur)
	if err != nil {
		return 0, err
	}
	switch v.Kind {
	case VKInt:
		return int(v.I64), nil
	case VKUint, VKBytesLen:
		return int(v.U64), nil
	default:
		return 0, fmt.Errorf("expected int, got %v", v.Kind)
	}
}

func EvalUint(ctx *Ctx, cur *Cursor, e Expr) (uint64, error) {
	v, err := e.Eval(ctx, cur)
	if err != nil {
		return 0, err
	}
	switch v.Kind {
	case VKUint, VKBytesLen:
		return v.U64, nil
	case VKInt:
		if v.I64 < 0 {
			return 0, fmt.Errorf("negative to uint: %d", v.I64)
		}
		return uint64(v.I64), nil
	default:
		return 0, fmt.Errorf("expected uint, got %v", v.Kind)
	}
}

func EvalBool(ctx *Ctx, cur *Cursor, e Expr) (bool, error) {
	v, err := e.Eval(ctx, cur)
	if err != nil {
		return false, err
	}
	if v.Kind != VKBool {
		return false, fmt.Errorf("expected bool, got %v", v.Kind)
	}
	return v.Bool, nil
}

func ParseExpr(s string) (Expr, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}
	p := parser{toks: toks}
	ex, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("unexpected token: %s", p.peek().text)
	}
	return ex, nil
}

//
// -------- Lexer / parser (tiny Pratt) --------
//

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokInt
	tokLParen
	tokRParen
	tokOp
)

type token struct {
	kind tokKind
	text string
	u64  uint64
}

func lex(s string) ([]token, error) {
	s = strings.TrimSpace(s)
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		if isSpace(c) {
			i++
			continue
		}
		switch c {
		case '(':
			toks = append(toks, token{kind: tokLParen, text: "("})
			i++
			continue
		case ')':
			toks = append(toks, token{kind: tokRParen, text: ")"})
			i++
			continue
		}

		if op, n := matchOp(s[i:]); n > 0 {
			toks = append(toks, token{kind: tokOp, text: op})
			i += n
			continue
		}
		if isDigit(c) || (c == '0' && i+1 < len(s) && (s[i+1] == 'x' || s[i+1] == 'X')) {
			t, n, err := scanInt(s[i:])
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
			i += n
			continue
		}
		if isIdentStart(c) {
			j := i + 1
			for j < len(s) && isIdentCont(s[j]) {
				j++
			}
			toks = append(toks, token{kind: tokIdent, text: s[i:j]})
			i = j
			continue
		}
		return nil, fmt.Errorf("unexpected char %q at %d", c, i)
	}
	toks = append(toks, token{kind: tokEOF})
	return toks, nil
}

func matchOp(s string) (string, int) {
	ops := []string{"&&", "||", "==", "!=", "<=", ">=", "+", "-", "*", "/", "%", "<", ">", "!"}
	for _, op := range ops {
		if strings.HasPrefix(s, op) {
			return op, len(op)
		}
	}
	return "", 0
}

func scanInt(s string) (token, int, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		j := 2
		for j < len(s) && isHex(s[j]) {
			j++
		}
		u, err := strconv.ParseUint(s[2:j], 16, 64)
		if err != nil {
			return token{}, 0, err
		}
		return token{kind: tokInt, text: s[:j], u64: u}, j, nil
	}
	j := 0
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	u, err := strconv.ParseUint(s[:j], 10, 64)
	if err != nil {
		return token{}, 0, err
	}
	return token{kind: tokInt, text: s[:j], u64: u}, j, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool   { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '.'
}
func isIdentCont(c byte) bool { return isIdentStart(c) || isDigit(c) }

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }

func prec(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=", "<", "<=", ">", ">=":
		return 3
	case "+", "-":
		return 4
	case "*", "/", "%":
		return 5
	}
	return 0
}

func (p *parser) parseExpr() (Expr, error) { return p.parseBinary(1) }

func (p *parser) parseBinary(minPrec int) (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tokOp {
			break
		}
		op := t.text
		opPrec := prec(op)
		if opPrec < minPrec {
			break
		}
		p.next()
		right, err := p.parseBinary(opPrec + 1)
		if err != nil {
			return nil, err
		}
		left = &binExpr{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	t := p.peek()
	if t.kind == tokOp && (t.text == "!" || t.text == "-") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &unExpr{op: t.text, x: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.next()
	switch t.kind {
	case tokInt:
		return &litExpr{u: t.u64, text: t.text}, nil
	case tokIdent:
		return &identExpr{name: t.text}, nil
	case tokLParen:
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, fmt.Errorf("expected ')'")
		}
		p.next()
		return e, nil
	default:
		return nil, fmt.Errorf("unexpected token %q", t.text)
	}
}

type litExpr struct {
	u    uint64
	text string
}

func (e *litExpr) Eval(ctx *Ctx, cur *Cursor) (Value, error) {
	return Value{Kind: VKUint, U64: e.u}, nil
}
func (e *litExpr) String() string { return e.text }

type identExpr struct{ name string }

func (e *identExpr) Eval(ctx *Ctx, cur *Cursor) (Value, error) {
	switch e.name {
	case "remaining":
		return Value{Kind: VKUint, U64: uint64(cur.Remaining())}, nil
	case "offset":
		return Value{Kind: VKUint, U64: uint64(cur.Offset())}, nil
	case "size":
		return Value{Kind: VKUint, U64: uint64(cur.Limit)}, nil
	}
	v, ok := ctx.Values[e.name]
	if !ok {
		return Value{}, fmt.Errorf("unknown identifier %q", e.name)
	}
	return v, nil
}
func (e *identExpr) String() string { return e.name }

type unExpr struct {
	op string
	x  Expr
}

func (e *unExpr) Eval(ctx *Ctx, cur *Cursor) (Value, error) {
	v, err := e.x.Eval(ctx, cur)
	if err != nil {
		return Value{}, err
	}
	switch e.op {
	case "!":
		b, err := toBool(v)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: VKBool, Bool: !b}, nil
	case "-":
		i, err := toInt64(v)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: VKInt, I64: -i}, nil
	default:
		return Value{}, fmt.Errorf("unknown unary op %q", e.op)
	}
}
func (e *unExpr) String() string { return e.op + e.x.String() }

type binExpr struct {
	op          string
	left, right Expr
}

func (e *binExpr) Eval(ctx *Ctx, cur *Cursor) (Value, error) {
	lv, err := e.left.Eval(ctx, cur)
	if err != nil {
		return Value{}, err
	}
	rv, err := e.right.Eval(ctx, cur)
	if err != nil {
		return Value{}, err
	}

	switch e.op {
	case "&&", "||":
		lb, err := toBool(lv)
		if err != nil {
			return Value{}, err
		}
		rb, err := toBool(rv)
		if err != nil {
			return Value{}, err
		}
		if e.op == "&&" {
			return Value{Kind: VKBool, Bool: lb && rb}, nil
		}
		return Value{Kind: VKBool, Bool: lb || rb}, nil

	case "==", "!=", "<", "<=", ">", ">=":
		li, lu, lIsU, err := toComparable(lv)
		if err != nil {
			return Value{}, err
		}
		ri, ru, rIsU, err := toComparable(rv)
		if err != nil {
			return Value{}, err
		}

		// Prefer uint compare when both non-negative and at least one is uint.
		if (lIsU || rIsU) && li >= 0 && ri >= 0 {
			switch e.op {
			case "==":
				return Value{Kind: VKBool, Bool: lu == ru}, nil
			case "!=":
				return Value{Kind: VKBool, Bool: lu != ru}, nil
			case "<":
				return Value{Kind: VKBool, Bool: lu < ru}, nil
			case "<=":
				return Value{Kind: VKBool, Bool: lu <= ru}, nil
			case ">":
				return Value{Kind: VKBool, Bool: lu > ru}, nil
			case ">=":
				return Value{Kind: VKBool, Bool: lu >= ru}, nil
			}
		}

		// Signed compare fallback
		switch e.op {
		case "==":
			return Value{Kind: VKBool, Bool: li == ri}, nil
		case "!=":
			return Value{Kind: VKBool, Bool: li != ri}, nil
		case "<":
			return Value{Kind: VKBool, Bool: li < ri}, nil
		case "<=":
			return Value{Kind: VKBool, Bool: li <= ri}, nil
		case ">":
			return Value{Kind: VKBool, Bool: li > ri}, nil
		case ">=":
			return Value{Kind: VKBool, Bool: li >= ri}, nil
		}

	case "+", "-", "*", "/", "%":
		li, err := toInt64(lv)
		if err != nil {
			return Value{}, err
		}
		ri, err := toInt64(rv)
		if err != nil {
			return Value{}, err
		}
		switch e.op {
		case "+":
			return Value{Kind: VKInt, I64: li + ri}, nil
		case "-":
			return Value{Kind: VKInt, I64: li - ri}, nil
		case "*":
			return Value{Kind: VKInt, I64: li * ri}, nil
		case "/":
			if ri == 0 {
				return Value{}, errors.New("division by zero")
			}
			return Value{Kind: VKInt, I64: li / ri}, nil
		case "%":
			if ri == 0 {
				return Value{}, errors.New("mod by zero")
			}
			return Value{Kind: VKInt, I64: li % ri}, nil
		}
	}

	return Value{}, fmt.Errorf("unknown op %q", e.op)
}
func (e *binExpr) String() string {
	return "(" + e.left.String() + " " + e.op + " " + e.right.String() + ")"
}

func toBool(v Value) (bool, error) {
	switch v.Kind {
	case VKBool:
		return v.Bool, nil
	case VKInt:
		return v.I64 != 0, nil
	case VKUint, VKBytesLen:
		return v.U64 != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %v to bool", v.Kind)
	}
}

func toInt64(v Value) (int64, error) {
	switch v.Kind {
	case VKInt:
		return v.I64, nil
	case VKUint, VKBytesLen:
		return int64(v.U64), nil
	case VKBool:
		if v.Bool {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("cannot convert %v to int64", v.Kind)
	}
}

func toComparable(v Value) (i int64, u uint64, isU bool, err error) {
	switch v.Kind {
	case VKInt:
		return v.I64, uint64(v.I64), false, nil
	case VKUint, VKBytesLen:
		return int64(v.U64), v.U64, true, nil
	case VKBool:
		if v.Bool {
			return 1, 1, true, nil
		}
		return 0, 0, true, nil
	default:
		return 0, 0, false, fmt.Errorf("cannot compare kind %v", v.Kind)
	}
}

//
// -------- Types / numeric decode --------
//

type Kind int

const (
	KindNum Kind = iota
	KindBytes
)

type TypeInfo struct {
	Kind   Kind
	Size   int
	Signed bool
}

var typeInfos = map[string]TypeInfo{
	"u8":    {Kind: KindNum, Size: 1, Signed: false},
	"i8":    {Kind: KindNum, Size: 1, Signed: true},
	"u16":   {Kind: KindNum, Size: 2, Signed: false},
	"i16":   {Kind: KindNum, Size: 2, Signed: true},
	"u24":   {Kind: KindNum, Size: 3, Signed: false},
	"u32":   {Kind: KindNum, Size: 4, Signed: false},
	"i32":   {Kind: KindNum, Size: 4, Signed: true},
	"u64":   {Kind: KindNum, Size: 8, Signed: false},
	"i64":   {Kind: KindNum, Size: 8, Signed: true},
	"bytes": {Kind: KindBytes, Size: 0, Signed: false},
}

func readNumber(order binary.ByteOrder, b []byte, signed bool) (int64, uint64) {
	switch len(b) {
	case 1:
		u := uint64(b[0])
		if signed {
			return int64(int8(b[0])), u
		}
		return int64(u), u
	case 2:
		u := uint64(order.Uint16(b))
		if signed {
			return int64(int16(u)), u
		}
		return int64(u), u
	case 4:
		u := uint64(order.Uint32(b))
		if signed {
			return int64(int32(u)), u
		}
		return int64(u), u
	case 8:
		u := order.Uint64(b)
		if signed {
			return int64(u), u
		}
		return int64(u), u
	default:
		var u uint64
		for _, x := range b {
			u = (u << 8) | uint64(x)
		}
		if signed {
			return int64(u), u
		}
		return int64(u), u
	}
}

func printableASCII(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c >= 0x20 && c <= 0x7E {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('.')
		}
	}
	return sb.String()
}

func formatBytesValue(f *FieldNode, b []byte) string {
	switch f.ValueFmt {
	case "ascii":
		if s := printableASCIITrim(b); s != "" {
			return s
		}
		return fmt.Sprintf("len=%d", len(b))
	case "hex":
		return hexBytesCompact(b)
	case "bcd":
		if s := bcdDigits(b); s != "" {
			return s
		}
		return hexBytesCompact(b)
	case "datetime_bcd6", "datetime_6bcd", "date_bcd6":
		if s, ok := parseBCDDateTime6(b); ok {
			return s
		}
		return hexBytesCompact(b)
	default:
		return fmt.Sprintf("len=%d", len(b))
	}
}

func formatNumericValue(f *FieldNode, i64 int64, u64 uint64, signed bool) string {
	switch f.ValueFmt {
	case "dec":
		if signed {
			return fmt.Sprintf("%d", i64)
		}
		return fmt.Sprintf("%d", u64)
	case "hex":
		if signed {
			return fmt.Sprintf("0x%X", uint64(i64))
		}
		return fmt.Sprintf("0x%X", u64)
	case "unix_ms", "datetime_unix_ms":
		if signed {
			return formatUnixMillis(i64)
		}
		if u64 > uint64(^uint64(0)>>1) {
			return fmt.Sprintf("%d", u64)
		}
		return formatUnixMillis(int64(u64))
	case "unix_s", "datetime_unix_s":
		if signed {
			return formatUnixSeconds(i64)
		}
		if u64 > uint64(^uint64(0)>>1) {
			return fmt.Sprintf("%d", u64)
		}
		return formatUnixSeconds(int64(u64))
	case "coord_1e7":
		if signed {
			return formatCoord(float64(i64) / 1e7)
		}
		return formatCoord(float64(u64) / 1e7)
	case "coord_1e6":
		if signed {
			return formatCoord(float64(i64) / 1e6)
		}
		return formatCoord(float64(u64) / 1e6)
	case "coord_gt06":
		return formatCoord(float64(u64) / 1800000.0)
	}

	if signed {
		return fmt.Sprintf("%d", i64)
	}
	return fmt.Sprintf("0x%X", u64)
}

func printableASCIITrim(b []byte) string {
	start := 0
	for start < len(b) && !isASCIIPrintable(b[start]) {
		start++
	}
	end := len(b)
	for end > start && !isASCIIPrintable(b[end-1]) {
		end--
	}
	if start >= end {
		return ""
	}

	var sb strings.Builder
	for _, c := range b[start:end] {
		if isASCIIPrintable(c) {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func isASCIIPrintable(b byte) bool {
	return b >= 0x20 && b <= 0x7E
}

func hexBytesCompact(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(len(b) * 2)
	const hexd = "0123456789ABCDEF"
	for _, v := range b {
		sb.WriteByte(hexd[v>>4])
		sb.WriteByte(hexd[v&0x0F])
	}
	return sb.String()
}

func bcdDigits(b []byte) string {
	var sb strings.Builder
	for _, v := range b {
		hi := (v >> 4) & 0x0F
		lo := v & 0x0F
		if hi <= 9 {
			sb.WriteByte('0' + hi)
		}
		if lo <= 9 {
			sb.WriteByte('0' + lo)
		}
	}
	return sb.String()
}

func parseBCDDateTime6(b []byte) (string, bool) {
	if len(b) != 6 {
		return "", false
	}
	var parts [6]int
	allBCD := true
	for _, v := range b {
		hi := int((v >> 4) & 0x0F)
		lo := int(v & 0x0F)
		if hi > 9 || lo > 9 {
			allBCD = false
			break
		}
	}
	for i := range b {
		if allBCD {
			hi := int((b[i] >> 4) & 0x0F)
			lo := int(b[i] & 0x0F)
			parts[i] = hi*10 + lo
			continue
		}
		if b[i] > 99 {
			return "", false
		}
		parts[i] = int(b[i])
	}
	year := 2000 + parts[0]
	if parts[1] < 1 || parts[1] > 12 || parts[2] < 1 || parts[2] > 31 || parts[3] > 23 || parts[4] > 59 || parts[5] > 59 {
		return "", false
	}
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", year, parts[1], parts[2], parts[3], parts[4], parts[5]), true
}

func formatUnixMillis(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05Z")
}

func formatUnixSeconds(sec int64) string {
	return time.Unix(sec, 0).UTC().Format("2006-01-02 15:04:05Z")
}

func formatCoord(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func uint16FromSpan(raw []byte, sp *Span, order binary.ByteOrder) uint16 {
	if sp == nil || sp.End-sp.Start != 2 || sp.Start < 0 || sp.End > len(raw) {
		return 0
	}
	return order.Uint16(raw[sp.Start:sp.End])
}

func uint32FromSpan(raw []byte, sp *Span, order binary.ByteOrder) uint32 {
	if sp == nil || sp.End-sp.Start != 4 || sp.Start < 0 || sp.End > len(raw) {
		return 0
	}
	return order.Uint32(raw[sp.Start:sp.End])
}

//
// -------- CRCs --------
//

// Teltonika: CRC-16/IBM (poly 0xA001, init 0x0000, refin/refout true)
func crc16IBM(data []byte) uint16 {
	var crc uint16 = 0x0000
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// GT06 "CRC-ITU" in docs corresponds to reflected CCITT (poly 0x8408) with init 0xFFFF and xorout 0xFFFF (CRC-16/X25)
func crc16X25(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

//
// -------- Const parsing --------
//

func parseIntAny(v any) (u uint64, i int64, isU bool, err error) {
	switch x := v.(type) {
	case int:
		if x < 0 {
			return 0, int64(x), false, nil
		}
		return uint64(x), int64(x), true, nil
	case int64:
		if x < 0 {
			return 0, x, false, nil
		}
		return uint64(x), x, true, nil
	case uint64:
		return x, int64(x), true, nil
	case string:
		return parseIntString(x)
	default:
		return 0, 0, false, fmt.Errorf("unsupported const type %T", v)
	}
}

func parseIntString(s string) (u uint64, i int64, isU bool, err error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		u, err := strconv.ParseUint(s[2:], 16, 64)
		if err != nil {
			return 0, 0, false, err
		}
		return u, int64(u), true, nil
	}
	if strings.HasPrefix(s, "-") {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, 0, false, err
		}
		return 0, i, false, nil
	}
	u, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, 0, false, err
	}
	return u, int64(u), true, nil
}

func inferPeekDefault(p *PeekNode) Value {
	if p == nil {
		return Value{Kind: VKUint, U64: 0}
	}
	if p.typeInfo.Kind == KindBytes && p.Check == "ascii_digits" {
		return Value{Kind: VKBool, Bool: false}
	}
	if p.typeInfo.Signed {
		return Value{Kind: VKInt, I64: 0}
	}
	return Value{Kind: VKUint, U64: 0}
}

func parsePeekDefault(p *PeekNode) (Value, error) {
	if p == nil {
		return Value{}, fmt.Errorf("internal: nil peek node")
	}
	if p.typeInfo.Kind == KindBytes && p.Check == "ascii_digits" {
		b, ok := p.Default.(bool)
		if !ok {
			return Value{}, fmt.Errorf("peek %q default for ascii_digits must be bool", p.Name)
		}
		return Value{Kind: VKBool, Bool: b}, nil
	}
	u, i, isU, err := parseIntAny(p.Default)
	if err != nil {
		return Value{}, fmt.Errorf("peek %q default: %w", p.Name, err)
	}
	if p.typeInfo.Signed && !isU {
		return Value{Kind: VKInt, I64: i}, nil
	}
	if p.typeInfo.Signed && isU {
		return Value{Kind: VKInt, I64: int64(u)}, nil
	}
	return Value{Kind: VKUint, U64: u}, nil
}

//
// -------- Small helpers --------
//

func nz(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func argString(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
