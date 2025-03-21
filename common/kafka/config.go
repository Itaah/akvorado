// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

// Package kafka exposes some common helpers for Kafka, including the
// configuration struture.
package kafka

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"

	"akvorado/common/helpers"

	"github.com/IBM/sarama"
)

// Configuration defines how we connect to a Kafka cluster.
type Configuration struct {
	// Topic defines the topic to write flows to.
	Topic string `validate:"required"`
	// Brokers is the list of brokers to connect to.
	Brokers []string `min=1,dive,validate:"listen"`
	// Version is the version of Kafka we assume to work
	Version Version
	// TLS defines TLS configuration
	TLS TLSAndSASLConfiguration
}

// TLSAndSASLConfiguration defines TLS configuration.
type TLSAndSASLConfiguration struct {
	helpers.TLSConfiguration `mapstructure:",squash" yaml:",inline"`
	// SASLUsername tells the SASL username
	SASLUsername string `validate:"required_with=SASLAlgorithm"`
	// SASLPassword tells the SASL password
	SASLPassword string `validate:"required_with=SASLAlgorithm SASLUsername"`
	// SASLMechanism tells the SASL algorithm
	SASLMechanism SASLMechanism `validate:"required_with=SASLUsername"`
}

// DefaultConfiguration represents the default configuration for connecting to Kafka.
func DefaultConfiguration() Configuration {
	return Configuration{
		Topic:   "flows",
		Brokers: []string{"127.0.0.1:9092"},
		Version: Version(sarama.V2_8_1_0),
		TLS: TLSAndSASLConfiguration{
			TLSConfiguration: helpers.TLSConfiguration{
				Enable: false,
				Verify: true,
			},
		},
	}
}

// Version represents a supported version of Kafka
type Version sarama.KafkaVersion

// UnmarshalText parses a version of Kafka
func (v *Version) UnmarshalText(text []byte) error {
	version, err := sarama.ParseKafkaVersion(string(text))
	if err != nil {
		return err
	}
	*v = Version(version)
	return nil
}

// String turns a Kafka version into a string
func (v Version) String() string {
	return sarama.KafkaVersion(v).String()
}

// MarshalText turns a Kafka version into a string
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// SASLMechanism defines an SASL algorithm
type SASLMechanism int

const (
	// SASLNone means no user authentication
	SASLNone SASLMechanism = iota
	// SASLPlain means user/password in plain text
	SASLPlain
	// SASLScramSHA256 enables SCRAM challenge with SHA256
	SASLScramSHA256
	// SASLScramSHA512 enables SCRAM challenge with SHA512
	SASLScramSHA512
)

// NewConfig returns a Sarama Kafka configuration ready to use.
func NewConfig(config Configuration) (*sarama.Config, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.KafkaVersion(config.Version)
	kafkaConfig.ClientID = fmt.Sprintf("akvorado-%s", helpers.AkvoradoVersion)
	tlsConfig, err := config.TLS.TLSConfiguration.MakeTLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		kafkaConfig.Net.TLS.Enable = true
		kafkaConfig.Net.TLS.Config = tlsConfig
		// SASL
		if config.TLS.SASLUsername != "" {
			kafkaConfig.Net.SASL.Enable = true
			kafkaConfig.Net.SASL.User = config.TLS.SASLUsername
			kafkaConfig.Net.SASL.Password = config.TLS.SASLPassword
			kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext
			if config.TLS.SASLMechanism == SASLScramSHA256 {
				kafkaConfig.Net.SASL.Handshake = true
				kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
				kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
					return &xdgSCRAMClient{HashGeneratorFcn: sha256.New}
				}
			}
			if config.TLS.SASLMechanism == SASLScramSHA512 {
				kafkaConfig.Net.SASL.Handshake = true
				kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
				kafkaConfig.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
					return &xdgSCRAMClient{HashGeneratorFcn: sha512.New}
				}
			}
		}
	}
	return kafkaConfig, nil
}
