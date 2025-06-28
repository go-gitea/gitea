// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

// cmdCert represents the available cert sub-command.
func cmdCert() *cli.Command {
	return &cli.Command{
		Name:  "cert",
		Usage: "Generate self-signed certificate",
		Description: `Generate a self-signed X.509 certificate for a TLS server.
Outputs to 'cert.pem' and 'key.pem' and will overwrite existing files.`,
		Action: runCert,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "host",
				Usage:    "Comma-separated hostnames and IPs to generate a certificate for",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "ecdsa-curve",
				Value: "",
				Usage: "ECDSA curve to use to generate a key. Valid values are P224, P256, P384, P521",
			},
			&cli.IntFlag{
				Name:  "rsa-bits",
				Value: 3072,
				Usage: "Size of RSA key to generate. Ignored if --ecdsa-curve is set",
			},
			&cli.StringFlag{
				Name:  "start-date",
				Value: "",
				Usage: "Creation date formatted as Jan 1 15:04:05 2011",
			},
			&cli.DurationFlag{
				Name:  "duration",
				Value: 365 * 24 * time.Hour,
				Usage: "Duration that certificate is valid for",
			},
			&cli.BoolFlag{
				Name:  "ca",
				Usage: "whether this cert should be its own Certificate Authority",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: "cert.pem",
				Usage: "Path to the file where there certificate will be saved",
			},
			&cli.StringFlag{
				Name:  "keyout",
				Value: "key.pem",
				Usage: "Path to the file where there certificate key will be saved",
			},
		},
	}
}

func publicKey(priv any) any {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv any) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			log.Fatalf("Unable to marshal ECDSA private key: %v", err)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func runCert(_ context.Context, c *cli.Command) error {
	var priv any
	var err error
	switch c.String("ecdsa-curve") {
	case "":
		priv, err = rsa.GenerateKey(rand.Reader, c.Int("rsa-bits"))
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		err = fmt.Errorf("unrecognized elliptic curve: %q", c.String("ecdsa-curve"))
	}
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	var notBefore time.Time
	if startDate := c.String("start-date"); startDate != "" {
		notBefore, err = time.Parse("Jan 2 15:04:05 2006", startDate)
		if err != nil {
			return fmt.Errorf("failed to parse creation date %w", err)
		}
	} else {
		notBefore = time.Now()
	}

	notAfter := notBefore.Add(c.Duration("duration"))

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
			CommonName:   "Gitea",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.SplitSeq(c.String("host"), ",")
	for h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if c.Bool("ca") {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certOut, err := os.Create(c.String("out"))
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", c.String("keyout"), err)
	}
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}
	err = certOut.Close()
	if err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}
	fmt.Fprintf(c.Writer, "Written cert to %s\n", c.String("out"))

	keyOut, err := os.OpenFile(c.String("keyout"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", c.String("keyout"), err)
	}
	err = pem.Encode(keyOut, pemBlockForKey(priv))
	if err != nil {
		return fmt.Errorf("failed to encode key: %w", err)
	}
	err = keyOut.Close()
	if err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	fmt.Fprintf(c.Writer, "Written key to %s\n", c.String("keyout"))
	return nil
}
