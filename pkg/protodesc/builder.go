package protodesc

import (
	"bytes"
	"compress/gzip"
	"reflect"
	"strconv"
	"strings"

	gogoproto "github.com/cosmos/gogoproto/proto"
	protov2 "google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// MsgEntry pairs a proto message name with a zero-value instance of the struct.
type MsgEntry struct {
	Name     string
	Instance interface{}
}

// MethodEntry describes one gRPC service method.
type MethodEntry struct {
	Name       string // e.g. "Deposit"
	InputType  string // fully qualified, e.g. ".funai.settlement.MsgDeposit"
	OutputType string // fully qualified, e.g. ".funai.settlement.MsgDepositResponse"
}

// ServiceEntry describes a gRPC service within a proto file.
type ServiceEntry struct {
	Name    string        // e.g. "Msg" or "Query"
	Methods []MethodEntry
}

// FileDescriptor holds everything needed to build and register a synthetic proto file.
type FileDescriptor struct {
	FileName    string         // e.g. "funai/settlement/types.proto"
	PackageName string         // e.g. "funai.settlement"
	Messages    []MsgEntry
	Services    []ServiceEntry
}

// BuildAndRegister creates a synthetic proto file descriptor from Go struct tags,
// gzips it, registers it with gogoproto (so HybridResolver can find services),
// and returns the gzipped bytes for use in Descriptor() methods.
func BuildAndRegister(fd FileDescriptor) []byte {
	gz := buildFileDescriptorGz(fd)
	gogoproto.RegisterFile(fd.FileName, gz)
	return gz
}

func buildFileDescriptorGz(fd FileDescriptor) []byte {
	var msgDescs []*descriptorpb.DescriptorProto
	for _, m := range fd.Messages {
		msgDescs = append(msgDescs, buildMsgDesc(m.Name, m.Instance))
	}

	var svcDescs []*descriptorpb.ServiceDescriptorProto
	for _, s := range fd.Services {
		var methods []*descriptorpb.MethodDescriptorProto
		for _, m := range s.Methods {
			methods = append(methods, &descriptorpb.MethodDescriptorProto{
				Name:       protov2.String(m.Name),
				InputType:  protov2.String(m.InputType),
				OutputType: protov2.String(m.OutputType),
			})
		}
		svcDescs = append(svcDescs, &descriptorpb.ServiceDescriptorProto{
			Name:   protov2.String(s.Name),
			Method: methods,
		})
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Name:        protov2.String(fd.FileName),
		Package:     protov2.String(fd.PackageName),
		Syntax:      protov2.String("proto3"),
		MessageType: msgDescs,
		Service:     svcDescs,
	}

	b, _ := protov2.Marshal(fdp)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(b)
	_ = gz.Close()
	return buf.Bytes()
}

func buildMsgDesc(name string, instance interface{}) *descriptorpb.DescriptorProto {
	t := reflect.TypeOf(instance)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var fields []*descriptorpb.FieldDescriptorProto
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag := sf.Tag.Get("protobuf")
		if tag == "" {
			continue
		}

		parts := strings.Split(tag, ",")
		if len(parts) < 2 {
			continue
		}

		wireType := parts[0]
		number, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		var typ descriptorpb.FieldDescriptorProto_Type
		switch wireType {
		case "bytes":
			if sf.Type.Kind() == reflect.String {
				typ = descriptorpb.FieldDescriptorProto_TYPE_STRING
			} else {
				typ = descriptorpb.FieldDescriptorProto_TYPE_BYTES
			}
		case "varint":
			goType := sf.Type
			if goType.Kind() == reflect.Ptr {
				goType = goType.Elem()
			}
			switch goType.Kind() {
			case reflect.Bool:
				typ = descriptorpb.FieldDescriptorProto_TYPE_BOOL
			case reflect.Uint64, reflect.Uint32:
				typ = descriptorpb.FieldDescriptorProto_TYPE_UINT64
			default:
				typ = descriptorpb.FieldDescriptorProto_TYPE_INT64
			}
		case "fixed64":
			typ = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
		case "fixed32":
			typ = descriptorpb.FieldDescriptorProto_TYPE_FLOAT
		default:
			typ = descriptorpb.FieldDescriptorProto_TYPE_BYTES
		}

		label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
		for _, p := range parts[2:] {
			if p == "rep" {
				label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
			}
		}

		fieldName := ""
		for _, p := range parts {
			if strings.HasPrefix(p, "name=") {
				fieldName = strings.TrimPrefix(p, "name=")
			}
		}
		if fieldName == "" {
			fieldName = sf.Name
		}

		fields = append(fields, &descriptorpb.FieldDescriptorProto{
			Name:   protov2.String(fieldName),
			Number: protov2.Int32(int32(number)),
			Type:   typ.Enum(),
			Label:  label.Enum(),
		})
	}

	return &descriptorpb.DescriptorProto{
		Name:  protov2.String(name),
		Field: fields,
	}
}
