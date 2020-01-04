---
date: "2020-01-04"
title: "TLS Certificate and Certificate Authority Setup"
slug: "tls-certificate-ca-setup"
weight: 17
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "TLS Certificate and CA Setup"
    weight: 16
    identifier: "tls-certificate-ca-setup"
---

# TLS Certificate and Certificate Authority Setup

If you deploy Gitea to internal network of your organization (also called intranet), and if you plan to [setup HTTPS on your Gitea](/en-us/https-setup/), you can setup your own [Certificate Authority (CA)](https://en.wikipedia.org/wiki/Certificate_authority), which is used to issue TLS certificates.

If instead you deploy Gitea to Internet (public-faced network), it is better to [use Let's Encrypt](/en-us/https-setup/#using-let-s-encrypt) or commercial CA of your choice (DigiCert, Verisign, Thawte, etc.).

## Preparation

Create directory structure (or tree) to store credentials (certificates and keys), replace `/home/user/ca` with your chosen root directory:

```
mkdir /home/user/ca
mkdir /home/user/ca/root/{certs,crl,newcerts,private,config}
mkdir /home/user/ca/intermediate/{certs,crl,newcerts,private,csr,crl,config}
chmod 700 /home/user/ca/root/private /home/user/ca/intermediate/private
touch /home/user/ca/root/index.txt /home/user/ca/intermediate/index.txt
echo 1000 | tee /home/user/ca/root/serial /home/user/ca/intermediate/serial
```

The Gitea source code comes with configuration for both root and intermediate CA, available at `contrib/openssl` directory. Copy `root.cnf` and `intermediate.cnf` to each corresponding `config/` directory.

Pay attention to `dir` option from `[ CA_default ]`. This contain root directory of your CA. Ensure that the option's value match one from the created tree above.

## Root CA

Switch to root CA directory:

```
cd /home/user/ca/root
```

Generate 4096-bit RSA private key, encrypted with AES-256; and make it read-only and only accessible to the file owner:

```
openssl genrsa -aes256 -out private/root.key.pem 4096
chmod 400 private/root.key.pem
```

Use the root key generated above to generate self-signed root certificate, with long expiry date (20 years are reasonable choice). Apply `v3_ca` extension. Set mode to read-only:

```
openssl req \
 -config config/root.cnf \
 -key -private/root.key.pem \
 -new -sha256 -x509 \
 -days 7300
 -extensions v3_ca \
 -out certs/root.cert.pem
chmod 444 certs/root.cert.pem
```

## Intermediate CA

Switch to intermediate CA directory:

```
cd /home/user/ca/intermediate
```

As with root CA key, generate 4096-bit RSA private key, encrypted with AES-256 and make it read only for the file owner:

```
openssl genrsa -aes256 -out private/intermediate.key.pem 4096
chmod 400 private/intermediate.key.pem
```

Create certificate signing request (CSR) using the key generated above. Ensure that *Distinguished Name* information are same as root CA, but *Common Name* must be different:

```
openssl req \
 -new -sha256 \
 -config config/intermediate.cnf \
 -key private/intermediate.key.pem \
 -out csr/intermediate.csr.pem
```

Sign the CSR to create intermediate CA. Its validity should be shorter than root CA (for example, 10 years). Apply `v3_intermediate_ca` extension:

```
openssl ca \
 -config ../root/config/openssl.cnf
 -days 3650 \
 -extensions v3_intermediate_ca \
 -notext -md sha256 \
 -in csr/intermediate.csr.pem \
 -out certs/intermediate.cert.pem
chmod 444 certs/intermediate.cert.pem
```

Verify *chain of trust* between root and intermediate CA. It should return `OK`, which indicates that the trust chain is intact:

```
openssl verify -CAfile ../root/certs/root.cert.pem
```

Create CA bundle by append **intermediate and root CA** (not vice versa). This will be used when verifying certificates signed by intermediate CA:

```
cat certs/intermediate.cert.pem ../root/certs/root.cert.pem > certs/gitea_ca.cert.pem
chmod 444 certs/gitea_ca.cert.pem
```

## Signing Certificates

Now the CA has been in place, you can now sign CSR requests to generate server and client certificates.

First, generate RSA private key. Since server and client certificates typically expires in a year or less, use 2048 bit key instead:

```
cd /etc/ssl
openssl genrsa -out private/private.key.pem
```

Create the CSR with the private key generated above. For *Common Name* entry, use fully-qualified domain name (for server certificates) or any unique identifier, e.g. database username or email address (for client certificates):

```
openssl req \
 -new -sha256 \
 -config openssl.cnf \
 -key private/private.key.pem \
 -out mycert.csr.pem
```

Copy the CSR to `csr/` directory of intermediate CA (this will be `/home/user/ca/intermediate/csr`, change it if different).

As CA, sign the CSR. Depending on the certificate purpose, either apply `server_cert` extension (for server certificates) or `usr_cert` (for client certificates):

```
cd /home/user/ca/intermediate
openssl ca \
 -config config/intermediate.cnf \
 -extensions server_cert \
 -days 100 \
 -notext -md sha256 \
 -in csr/mycert.csr.pem \
 -out certs/mycert.cert.pem
```

Verify the chain of trust against CA bundle:

```
openssl verify -CAfile certs/gitea_ca.cert.pem certs/mycert.cert.pem
```

## Deploying CA

Now you can send the resulting certificate and CA bundle to each servers and clients that might use it. Ensure that the CA bundle have been added to CA store of your system. Refer to your system's documentation for details.

Also, if you use the certificate for enabling HTTPS on your Gitea instance, make sure to add the CA bundle to client's browser keystore.
