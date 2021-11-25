// Code generated by easyjson for marshaling/unmarshaling. DO NOT EDIT.

package elastic

import (
	json "encoding/json"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson1ed00e60DecodeGithubComOlivereElasticV7(in *jlexer.Lexer, out *bulkUpdateRequestCommandOp) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "_index":
			out.Index = string(in.String())
		case "_type":
			out.Type = string(in.String())
		case "_id":
			out.Id = string(in.String())
		case "parent":
			out.Parent = string(in.String())
		case "retry_on_conflict":
			if in.IsNull() {
				in.Skip()
				out.RetryOnConflict = nil
			} else {
				if out.RetryOnConflict == nil {
					out.RetryOnConflict = new(int)
				}
				*out.RetryOnConflict = int(in.Int())
			}
		case "routing":
			out.Routing = string(in.String())
		case "version":
			out.Version = int64(in.Int64())
		case "version_type":
			out.VersionType = string(in.String())
		case "if_seq_no":
			if in.IsNull() {
				in.Skip()
				out.IfSeqNo = nil
			} else {
				if out.IfSeqNo == nil {
					out.IfSeqNo = new(int64)
				}
				*out.IfSeqNo = int64(in.Int64())
			}
		case "if_primary_term":
			if in.IsNull() {
				in.Skip()
				out.IfPrimaryTerm = nil
			} else {
				if out.IfPrimaryTerm == nil {
					out.IfPrimaryTerm = new(int64)
				}
				*out.IfPrimaryTerm = int64(in.Int64())
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1ed00e60EncodeGithubComOlivereElasticV7(out *jwriter.Writer, in bulkUpdateRequestCommandOp) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Index != "" {
		const prefix string = ",\"_index\":"
		first = false
		out.RawString(prefix[1:])
		out.String(string(in.Index))
	}
	if in.Type != "" {
		const prefix string = ",\"_type\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Type))
	}
	if in.Id != "" {
		const prefix string = ",\"_id\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Id))
	}
	if in.Parent != "" {
		const prefix string = ",\"parent\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Parent))
	}
	if in.RetryOnConflict != nil {
		const prefix string = ",\"retry_on_conflict\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int(int(*in.RetryOnConflict))
	}
	if in.Routing != "" {
		const prefix string = ",\"routing\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Routing))
	}
	if in.Version != 0 {
		const prefix string = ",\"version\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int64(int64(in.Version))
	}
	if in.VersionType != "" {
		const prefix string = ",\"version_type\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.VersionType))
	}
	if in.IfSeqNo != nil {
		const prefix string = ",\"if_seq_no\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int64(int64(*in.IfSeqNo))
	}
	if in.IfPrimaryTerm != nil {
		const prefix string = ",\"if_primary_term\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int64(int64(*in.IfPrimaryTerm))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v bulkUpdateRequestCommandOp) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson1ed00e60EncodeGithubComOlivereElasticV7(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v bulkUpdateRequestCommandOp) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson1ed00e60EncodeGithubComOlivereElasticV7(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *bulkUpdateRequestCommandOp) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson1ed00e60DecodeGithubComOlivereElasticV7(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *bulkUpdateRequestCommandOp) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson1ed00e60DecodeGithubComOlivereElasticV7(l, v)
}
func easyjson1ed00e60DecodeGithubComOlivereElasticV71(in *jlexer.Lexer, out *bulkUpdateRequestCommandData) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "detect_noop":
			if in.IsNull() {
				in.Skip()
				out.DetectNoop = nil
			} else {
				if out.DetectNoop == nil {
					out.DetectNoop = new(bool)
				}
				*out.DetectNoop = bool(in.Bool())
			}
		case "doc":
			if m, ok := out.Doc.(easyjson.Unmarshaler); ok {
				m.UnmarshalEasyJSON(in)
			} else if m, ok := out.Doc.(json.Unmarshaler); ok {
				_ = m.UnmarshalJSON(in.Raw())
			} else {
				out.Doc = in.Interface()
			}
		case "doc_as_upsert":
			if in.IsNull() {
				in.Skip()
				out.DocAsUpsert = nil
			} else {
				if out.DocAsUpsert == nil {
					out.DocAsUpsert = new(bool)
				}
				*out.DocAsUpsert = bool(in.Bool())
			}
		case "script":
			if m, ok := out.Script.(easyjson.Unmarshaler); ok {
				m.UnmarshalEasyJSON(in)
			} else if m, ok := out.Script.(json.Unmarshaler); ok {
				_ = m.UnmarshalJSON(in.Raw())
			} else {
				out.Script = in.Interface()
			}
		case "scripted_upsert":
			if in.IsNull() {
				in.Skip()
				out.ScriptedUpsert = nil
			} else {
				if out.ScriptedUpsert == nil {
					out.ScriptedUpsert = new(bool)
				}
				*out.ScriptedUpsert = bool(in.Bool())
			}
		case "upsert":
			if m, ok := out.Upsert.(easyjson.Unmarshaler); ok {
				m.UnmarshalEasyJSON(in)
			} else if m, ok := out.Upsert.(json.Unmarshaler); ok {
				_ = m.UnmarshalJSON(in.Raw())
			} else {
				out.Upsert = in.Interface()
			}
		case "_source":
			if in.IsNull() {
				in.Skip()
				out.Source = nil
			} else {
				if out.Source == nil {
					out.Source = new(bool)
				}
				*out.Source = bool(in.Bool())
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1ed00e60EncodeGithubComOlivereElasticV71(out *jwriter.Writer, in bulkUpdateRequestCommandData) {
	out.RawByte('{')
	first := true
	_ = first
	if in.DetectNoop != nil {
		const prefix string = ",\"detect_noop\":"
		first = false
		out.RawString(prefix[1:])
		out.Bool(bool(*in.DetectNoop))
	}
	if in.Doc != nil {
		const prefix string = ",\"doc\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		if m, ok := in.Doc.(easyjson.Marshaler); ok {
			m.MarshalEasyJSON(out)
		} else if m, ok := in.Doc.(json.Marshaler); ok {
			out.Raw(m.MarshalJSON())
		} else {
			out.Raw(json.Marshal(in.Doc))
		}
	}
	if in.DocAsUpsert != nil {
		const prefix string = ",\"doc_as_upsert\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Bool(bool(*in.DocAsUpsert))
	}
	if in.Script != nil {
		const prefix string = ",\"script\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		if m, ok := in.Script.(easyjson.Marshaler); ok {
			m.MarshalEasyJSON(out)
		} else if m, ok := in.Script.(json.Marshaler); ok {
			out.Raw(m.MarshalJSON())
		} else {
			out.Raw(json.Marshal(in.Script))
		}
	}
	if in.ScriptedUpsert != nil {
		const prefix string = ",\"scripted_upsert\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Bool(bool(*in.ScriptedUpsert))
	}
	if in.Upsert != nil {
		const prefix string = ",\"upsert\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		if m, ok := in.Upsert.(easyjson.Marshaler); ok {
			m.MarshalEasyJSON(out)
		} else if m, ok := in.Upsert.(json.Marshaler); ok {
			out.Raw(m.MarshalJSON())
		} else {
			out.Raw(json.Marshal(in.Upsert))
		}
	}
	if in.Source != nil {
		const prefix string = ",\"_source\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Bool(bool(*in.Source))
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v bulkUpdateRequestCommandData) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson1ed00e60EncodeGithubComOlivereElasticV71(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v bulkUpdateRequestCommandData) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson1ed00e60EncodeGithubComOlivereElasticV71(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *bulkUpdateRequestCommandData) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson1ed00e60DecodeGithubComOlivereElasticV71(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *bulkUpdateRequestCommandData) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson1ed00e60DecodeGithubComOlivereElasticV71(l, v)
}
func easyjson1ed00e60DecodeGithubComOlivereElasticV72(in *jlexer.Lexer, out *bulkUpdateRequestCommand) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		in.Skip()
	} else {
		in.Delim('{')
		*out = make(bulkUpdateRequestCommand)
		for !in.IsDelim('}') {
			key := string(in.String())
			in.WantColon()
			var v1 bulkUpdateRequestCommandOp
			(v1).UnmarshalEasyJSON(in)
			(*out)[key] = v1
			in.WantComma()
		}
		in.Delim('}')
	}
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson1ed00e60EncodeGithubComOlivereElasticV72(out *jwriter.Writer, in bulkUpdateRequestCommand) {
	if in == nil && (out.Flags&jwriter.NilMapAsEmpty) == 0 {
		out.RawString(`null`)
	} else {
		out.RawByte('{')
		v2First := true
		for v2Name, v2Value := range in {
			if v2First {
				v2First = false
			} else {
				out.RawByte(',')
			}
			out.String(string(v2Name))
			out.RawByte(':')
			(v2Value).MarshalEasyJSON(out)
		}
		out.RawByte('}')
	}
}

// MarshalJSON supports json.Marshaler interface
func (v bulkUpdateRequestCommand) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson1ed00e60EncodeGithubComOlivereElasticV72(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v bulkUpdateRequestCommand) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson1ed00e60EncodeGithubComOlivereElasticV72(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *bulkUpdateRequestCommand) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson1ed00e60DecodeGithubComOlivereElasticV72(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *bulkUpdateRequestCommand) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson1ed00e60DecodeGithubComOlivereElasticV72(l, v)
}
