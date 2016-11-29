// Code generated by protoc-gen-go.
// source: google.golang.org/genproto/googleapis/api/serviceconfig/logging.proto
// DO NOT EDIT!

package google_api // import "google.golang.org/genproto/googleapis/api/serviceconfig"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// Logging configuration of the service.
//
// The following example shows how to configure logs to be sent to the
// producer and consumer projects. In the example,
// the `library.googleapis.com/activity_history` log is
// sent to both the producer and consumer projects, whereas
// the `library.googleapis.com/purchase_history` log is only sent to the
// producer project:
//
//     monitored_resources:
//     - type: library.googleapis.com/branch
//       labels:
//       - key: /city
//         description: The city where the library branch is located in.
//       - key: /name
//         description: The name of the branch.
//     logs:
//     - name: library.googleapis.com/activity_history
//       labels:
//       - key: /customer_id
//     - name: library.googleapis.com/purchase_history
//     logging:
//       producer_destinations:
//       - monitored_resource: library.googleapis.com/branch
//         logs:
//         - library.googleapis.com/activity_history
//         - library.googleapis.com/purchase_history
//       consumer_destinations:
//       - monitored_resource: library.googleapis.com/branch
//         logs:
//         - library.googleapis.com/activity_history
//
type Logging struct {
	// Logging configurations for sending logs to the producer project.
	// There can be multiple producer destinations, each one must have a
	// different monitored resource type. A log can be used in at most
	// one producer destination.
	ProducerDestinations []*Logging_LoggingDestination `protobuf:"bytes,1,rep,name=producer_destinations,json=producerDestinations" json:"producer_destinations,omitempty"`
	// Logging configurations for sending logs to the consumer project.
	// There can be multiple consumer destinations, each one must have a
	// different monitored resource type. A log can be used in at most
	// one consumer destination.
	ConsumerDestinations []*Logging_LoggingDestination `protobuf:"bytes,2,rep,name=consumer_destinations,json=consumerDestinations" json:"consumer_destinations,omitempty"`
}

func (m *Logging) Reset()                    { *m = Logging{} }
func (m *Logging) String() string            { return proto.CompactTextString(m) }
func (*Logging) ProtoMessage()               {}
func (*Logging) Descriptor() ([]byte, []int) { return fileDescriptor9, []int{0} }

func (m *Logging) GetProducerDestinations() []*Logging_LoggingDestination {
	if m != nil {
		return m.ProducerDestinations
	}
	return nil
}

func (m *Logging) GetConsumerDestinations() []*Logging_LoggingDestination {
	if m != nil {
		return m.ConsumerDestinations
	}
	return nil
}

// Configuration of a specific logging destination (the producer project
// or the consumer project).
type Logging_LoggingDestination struct {
	// The monitored resource type. The type must be defined in
	// [Service.monitored_resources][google.api.Service.monitored_resources] section.
	MonitoredResource string `protobuf:"bytes,3,opt,name=monitored_resource,json=monitoredResource" json:"monitored_resource,omitempty"`
	// Names of the logs to be sent to this destination. Each name must
	// be defined in the [Service.logs][google.api.Service.logs] section.
	Logs []string `protobuf:"bytes,1,rep,name=logs" json:"logs,omitempty"`
}

func (m *Logging_LoggingDestination) Reset()                    { *m = Logging_LoggingDestination{} }
func (m *Logging_LoggingDestination) String() string            { return proto.CompactTextString(m) }
func (*Logging_LoggingDestination) ProtoMessage()               {}
func (*Logging_LoggingDestination) Descriptor() ([]byte, []int) { return fileDescriptor9, []int{0, 0} }

func init() {
	proto.RegisterType((*Logging)(nil), "google.api.Logging")
	proto.RegisterType((*Logging_LoggingDestination)(nil), "google.api.Logging.LoggingDestination")
}

func init() {
	proto.RegisterFile("google.golang.org/genproto/googleapis/api/serviceconfig/logging.proto", fileDescriptor9)
}

var fileDescriptor9 = []byte{
	// 254 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x9c, 0x90, 0xcf, 0x4a, 0x03, 0x31,
	0x10, 0x87, 0xd9, 0x56, 0x94, 0x46, 0x11, 0x0c, 0x0a, 0xa5, 0xa7, 0xc5, 0x83, 0xf4, 0x62, 0x02,
	0xfa, 0x06, 0x45, 0x0f, 0x82, 0x87, 0xb2, 0x17, 0x0f, 0x1e, 0x4a, 0xcc, 0xc6, 0x61, 0x60, 0x77,
	0x26, 0x24, 0x59, 0x9f, 0xc6, 0x87, 0x95, 0x6e, 0x76, 0xed, 0xa2, 0x27, 0x3d, 0x25, 0xe4, 0xf7,
	0xe5, 0x9b, 0x3f, 0xe2, 0x11, 0x98, 0xa1, 0x71, 0x0a, 0xb8, 0x31, 0x04, 0x8a, 0x03, 0x68, 0x70,
	0xe4, 0x03, 0x27, 0xd6, 0x39, 0x32, 0x1e, 0xa3, 0x36, 0x1e, 0x75, 0x74, 0xe1, 0x03, 0xad, 0xb3,
	0x4c, 0xef, 0x08, 0xba, 0x61, 0x00, 0x24, 0x50, 0x3d, 0x2a, 0xc5, 0xa0, 0x31, 0x1e, 0x57, 0x4f,
	0xff, 0x55, 0x1a, 0x22, 0x4e, 0x26, 0x21, 0x53, 0xcc, 0xda, 0xeb, 0xcf, 0x99, 0x38, 0x79, 0xce,
	0x85, 0xe4, 0xab, 0xb8, 0xf2, 0x81, 0xeb, 0xce, 0xba, 0xb0, 0xab, 0x5d, 0x4c, 0x48, 0x19, 0x5d,
	0x16, 0xe5, 0x7c, 0x7d, 0x7a, 0x77, 0xa3, 0x0e, 0x2d, 0xa8, 0xe1, 0xcf, 0x78, 0x3e, 0x1c, 0xf0,
	0xea, 0x72, 0x94, 0x4c, 0x1e, 0xe3, 0x5e, 0x6e, 0x99, 0x62, 0xd7, 0xfe, 0x94, 0xcf, 0xfe, 0x26,
	0x1f, 0x25, 0x53, 0xf9, 0xea, 0x45, 0xc8, 0xdf, 0xac, 0xbc, 0x15, 0xb2, 0x65, 0xc2, 0xc4, 0xc1,
	0xd5, 0xbb, 0xe0, 0x22, 0x77, 0xc1, 0xba, 0xe5, 0xbc, 0x2c, 0xd6, 0x8b, 0xea, 0xe2, 0x3b, 0xa9,
	0x86, 0x40, 0x4a, 0x71, 0xd4, 0x30, 0xe4, 0x69, 0x17, 0x55, 0x7f, 0xdf, 0x94, 0xe2, 0xdc, 0x72,
	0x3b, 0xe9, 0x6d, 0x73, 0x36, 0x14, 0xda, 0xee, 0xd7, 0xb7, 0x2d, 0xde, 0x8e, 0xfb, 0x3d, 0xde,
	0x7f, 0x05, 0x00, 0x00, 0xff, 0xff, 0xfc, 0x54, 0xda, 0x29, 0xe7, 0x01, 0x00, 0x00,
}
