package generator

import (
	"regexp"
	"sort"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
)

const jsonSerializer = "json"

var mediaTypeNames = map[*regexp.Regexp]string{
	regexp.MustCompile("application/.*json"):                jsonSerializer,
	regexp.MustCompile("application/.*yaml"):                "yaml",
	regexp.MustCompile("application/.*protobuf"):            "protobuf",
	regexp.MustCompile("application/.*capnproto"):           "capnproto",
	regexp.MustCompile("application/.*thrift"):              "thrift",
	regexp.MustCompile("(?:application|text)/.*xml"):        "xml",
	regexp.MustCompile("text/.*markdown"):                   "markdown",
	regexp.MustCompile("text/.*html"):                       "html",
	regexp.MustCompile("text/.*csv"):                        "csv",
	regexp.MustCompile("text/.*tsv"):                        "tsv",
	regexp.MustCompile("text/.*javascript"):                 "js",
	regexp.MustCompile("text/.*css"):                        "css",
	regexp.MustCompile("text/.*plain"):                      "txt",
	regexp.MustCompile("application/.*octet-stream"):        "bin",
	regexp.MustCompile("application/.*tar"):                 "tar",
	regexp.MustCompile("application/.*gzip"):                "gzip",
	regexp.MustCompile("application/.*gz"):                  "gzip",
	regexp.MustCompile("application/.*raw-stream"):          "bin",
	regexp.MustCompile("application/x-www-form-urlencoded"): "urlform",
	regexp.MustCompile("application/javascript"):            "txt",
	regexp.MustCompile("multipart/form-data"):               "multipartform",
	regexp.MustCompile("image/.*"):                          "bin",
	regexp.MustCompile("audio/.*"):                          "bin",
	regexp.MustCompile("application/pdf"):                   "bin",
}

var knownProducers = map[string]string{
	jsonSerializer:  "runtime.JSONProducer()",
	"yaml":          "yamlpc.YAMLProducer()",
	"xml":           "runtime.XMLProducer()",
	"txt":           "runtime.TextProducer()",
	"bin":           "runtime.ByteStreamProducer()",
	"urlform":       "runtime.DiscardProducer",
	"multipartform": "runtime.DiscardProducer",
}

var knownConsumers = map[string]string{
	jsonSerializer:  "runtime.JSONConsumer()",
	"yaml":          "yamlpc.YAMLConsumer()",
	"xml":           "runtime.XMLConsumer()",
	"txt":           "runtime.TextConsumer()",
	"bin":           "runtime.ByteStreamConsumer()",
	"urlform":       "runtime.DiscardConsumer",
	"multipartform": "runtime.DiscardConsumer",
}

func wellKnownMime(tn string) (string, bool) {
	for k, v := range mediaTypeNames {
		if k.MatchString(tn) {
			return v, true
		}
	}
	return "", false
}

func mediaMime(orig string) string {
	return strings.SplitN(orig, ";", 2)[0]
}

func mediaParameters(orig string) string {
	parts := strings.SplitN(orig, ";", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (a *appGenerator) makeSerializers(mediaTypes []string, known func(string) (string, bool)) (GenSerGroups, bool) {
	supportsJSON := false
	uniqueSerializers := make(map[string]*GenSerializer, len(mediaTypes))
	uniqueSerializerGroups := make(map[string]*GenSerGroup, len(mediaTypes))

	// build all required serializers
	for _, media := range mediaTypes {
		key := mediaMime(media)
		nm, ok := wellKnownMime(key)
		if !ok {
			// keep this serializer named, even though its implementation is empty (cf. #1557)
			nm = key
		}
		name := swag.ToJSONName(nm)
		impl, _ := known(name)

		ser, ok := uniqueSerializers[key]
		if !ok {
			ser = &GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           name,
				MediaType:      key,
				Implementation: impl,
				Parameters:     []string{},
			}
			uniqueSerializers[key] = ser
		}
		// provide all known parameters (currently unused by codegen templates)
		if params := strings.TrimSpace(mediaParameters(media)); params != "" {
			found := false
			for _, p := range ser.Parameters {
				if params == p {
					found = true
					break
				}
			}
			if !found {
				ser.Parameters = append(ser.Parameters, params)
			}
		}

		uniqueSerializerGroups[name] = &GenSerGroup{
			GenSerializer: GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           name,
				Implementation: impl,
			},
		}
	}

	if len(uniqueSerializers) == 0 {
		impl, _ := known(jsonSerializer)
		uniqueSerializers[runtime.JSONMime] = &GenSerializer{
			AppName:        a.Name,
			ReceiverName:   a.Receiver,
			Name:           jsonSerializer,
			MediaType:      runtime.JSONMime,
			Implementation: impl,
			Parameters:     []string{},
		}
		uniqueSerializerGroups[jsonSerializer] = &GenSerGroup{
			GenSerializer: GenSerializer{
				AppName:        a.Name,
				ReceiverName:   a.Receiver,
				Name:           jsonSerializer,
				Implementation: impl,
			},
		}
		supportsJSON = true
	}

	// group serializers by consumer/producer to serve several mime media types
	serializerGroups := make(GenSerGroups, 0, len(uniqueSerializers))

	for _, group := range uniqueSerializerGroups {
		if group.Name == jsonSerializer {
			supportsJSON = true
		}
		serializers := make(GenSerializers, 0, len(uniqueSerializers))
		for _, ser := range uniqueSerializers {
			if group.Name == ser.Name {
				sort.Strings(ser.Parameters)
				serializers = append(serializers, *ser)
			}
		}
		sort.Sort(serializers)
		group.AllSerializers = serializers // provides the full list of mime media types for this serializer group
		serializerGroups = append(serializerGroups, *group)
	}
	sort.Sort(serializerGroups)
	return serializerGroups, supportsJSON
}

func (a *appGenerator) makeConsumes() (GenSerGroups, bool) {
	// builds a codegen struct from all consumes in the spec
	return a.makeSerializers(a.Analyzed.RequiredConsumes(), func(media string) (string, bool) {
		c, ok := knownConsumers[media]
		return c, ok
	})
}

func (a *appGenerator) makeProduces() (GenSerGroups, bool) {
	// builds a codegen struct from all produces in the spec
	return a.makeSerializers(a.Analyzed.RequiredProduces(), func(media string) (string, bool) {
		p, ok := knownProducers[media]
		return p, ok
	})
}
