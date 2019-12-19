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

func easyjson9de0fcbfDecodeGithubComOlivereElasticV7(in *jlexer.Lexer, out *bulkIndexRequestCommandOp) {
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
		key := in.UnsafeString()
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "_index":
			out.Index = string(in.String())
		case "_id":
			out.Id = string(in.String())
		case "_type":
			out.Type = string(in.String())
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
			if in.IsNull() {
				in.Skip()
				out.Version = nil
			} else {
				if out.Version == nil {
					out.Version = new(int64)
				}
				*out.Version = int64(in.Int64())
			}
		case "version_type":
			out.VersionType = string(in.String())
		case "pipeline":
			out.Pipeline = string(in.String())
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
func easyjson9de0fcbfEncodeGithubComOlivereElasticV7(out *jwriter.Writer, in bulkIndexRequestCommandOp) {
	out.RawByte('{')
	first := true
	_ = first
	if in.Index != "" {
		const prefix string = ",\"_index\":"
		first = false
		out.RawString(prefix[1:])
		out.String(string(in.Index))
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
	if in.Version != nil {
		const prefix string = ",\"version\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.Int64(int64(*in.Version))
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
	if in.Pipeline != "" {
		const prefix string = ",\"pipeline\":"
		if first {
			first = false
			out.RawString(prefix[1:])
		} else {
			out.RawString(prefix)
		}
		out.String(string(in.Pipeline))
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
func (v bulkIndexRequestCommandOp) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson9de0fcbfEncodeGithubComOlivereElasticV7(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v bulkIndexRequestCommandOp) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson9de0fcbfEncodeGithubComOlivereElasticV7(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *bulkIndexRequestCommandOp) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson9de0fcbfDecodeGithubComOlivereElasticV7(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *bulkIndexRequestCommandOp) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson9de0fcbfDecodeGithubComOlivereElasticV7(l, v)
}
func easyjson9de0fcbfDecodeGithubComOlivereElasticV71(in *jlexer.Lexer, out *bulkIndexRequestCommand) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		in.Skip()
	} else {
		in.Delim('{')
		if !in.IsDelim('}') {
			*out = make(bulkIndexRequestCommand)
		} else {
			*out = nil
		}
		for !in.IsDelim('}') {
			key := string(in.String())
			in.WantColon()
			var v1 bulkIndexRequestCommandOp
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
func easyjson9de0fcbfEncodeGithubComOlivereElasticV71(out *jwriter.Writer, in bulkIndexRequestCommand) {
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
func (v bulkIndexRequestCommand) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson9de0fcbfEncodeGithubComOlivereElasticV71(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v bulkIndexRequestCommand) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson9de0fcbfEncodeGithubComOlivereElasticV71(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *bulkIndexRequestCommand) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson9de0fcbfDecodeGithubComOlivereElasticV71(&r, v)
	return r.Error()
}

// UnmarshalEasyJSON supports easyjson.Unmarshaler interface
func (v *bulkIndexRequestCommand) UnmarshalEasyJSON(l *jlexer.Lexer) {
	easyjson9de0fcbfDecodeGithubComOlivereElasticV71(l, v)
}
