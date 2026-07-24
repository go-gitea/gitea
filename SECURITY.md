# Reporting security issues

The Gitea maintainers take security seriously.

If you discover a security issue, please bring it to their attention right away!

Previous vulnerabilities are listed at https://about.gitea.com/security.

## Supported Versions

Security fixes follow our [support policy](docs/release-management.md#end-of-life-eol): only the most recent major release and the `main` branch receive them.

Reports must be filed against a supported version. Before reporting, verify that the issue reproduces on a supported version; any report that only affects an unsupported version will be closed as out of scope.

## Reporting a Vulnerability

Please **DO NOT** file a public issue. Instead, report the vulnerability privately by opening a [GitHub security advisory](https://github.com/go-gitea/gitea/security/advisories/new), or optionally by sending your report to `security@gitea.io`.
If reporting via GitHub, open an advisory per found issue. You can group the found issues if they relate to a single area (same feature, same fault present in multiple routes) but do not group unrelated reports into one as it makes it much harder to assess and track for duplicates.
Do not use LLM to write the reports. It's meant for people to read so take your time and use your own words to write it. If you have used LLM to find the bug, be transparent about it and verify that it is not hallucination before submitting the report.

If your report turns out to be a duplicate, you will be credited in the original report.

## Protecting Security Information

Due to the sensitive nature of security information, you can use the below GPG public key to encrypt your mail body.

The PGP key is valid until July 4, 2026.

```
Key ID: 6FCD2D5B
Key Type: RSA
Expires: 7/23/2027
Key Size: 4096/4096
Fingerprint: 3DE0 3D1E 144A 7F06 9359 99DC AAFD 2381 6FCD 2D5B
```

UserID: Gitea Security <security@gitea.io>

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGK1Z/4BEADFMqXA9DeeChmSxUjF0Be5sq99ZUhgrZjcN/wOzz0wuCJZC0l8
4uC+d6mfv7JpJYlzYzOK97/x5UguKHkYNZ6mm1G9KHaXmoIBDLKDzfPdJopVNv2r
OajijaE0uMCnMjadlg5pbhMLRQG8a9J32yyaz7ZEAw72Ab31fvvcA53NkuqO4j2w
k7dtFQzhbNOYV0VffQT90WDZdalYHB1JHyEQ+70U9OjVD5ggNYSzX98Eu3Hjn7V7
kqFrcAxr5TE1elf0IXJcuBJtFzQSTUGlQldKOHtGTGgGjj9r/FFAE5ioBgVD05bV
rEEgIMM/GqYaG/nbNpWE6P3mEc2Mnn3pZaRJL0LuF26TLjnqEcMMDp5iIhLdFzXR
3tMdtKgQFu+Mtzs3ipwWARYgHyU09RJsI2HeBx7RmZO/Xqrec763Z7zdJ7SpCn0Z
q+pHZl24JYR0Kf3T/ZiOC0cGd2QJqpJtg5J6S/OqfX9NH6MsCczO8pUC1N/aHH2X
CTme2nF56izORqDWKoiICteL3GpYsCV9nyCidcCmoQsS+DKvE86YhIhVIVWGRY2F
lzpAjnN9/KLtQroutrm+Ft0mdjDiJUeFVl1cOHDhoyfCsQh62HumoyZoZvqzQd6e
AbN11nq6aViMe2Q3je1AbiBnRnQSHxt1Tc8X4IshO3MQK1Sk7oPI6LA5oQARAQAB
tCJHaXRlYSBTZWN1cml0eSA8c2VjdXJpdHlAZ2l0ZWEuaW8+iQJXBBMBCABBAhsD
BQsJCAcCAiICBhUKCQgLAgQWAgMBAh4HAheAFiEEPeA9HhRKfwaTWZncqv0jgW/N
LVsFAmpi3IkFCQmOqAsACgkQqv0jgW/NLVsgyxAAuTyj4+PImOFr7ZkEJcseWUy+
gWQrJDagj+Ks2SKGneVroJ63q5Vngx44b7JNabauKel4IJJCScLdwA7k0DKFHPOM
rUS6aavW5+961CMQxJ13vWz7qxLxhI4FXle255xFSXgOo9T1W7+wtm/wtze8+sws
8XKAVBZGuVI1/KzVpE6bWxpIIYK79DC07NWGDAZzMbuO92TFHBRblXsicPWExBjC
/oFmT70YMcA0wOwU/YKYMg2eK2XYUr5+uYX6FnP7iUhDj1vZmwkF33Cj9v9D3Hv9
NzAquj97uLht4wL2sqsIgZd4Vc37uipPDFaddzTb/lon7lPbJPMLObcd4HjSTkIo
nEMM0wdOQZU7mIIgRM9o3mr/gyVigl3uARmUKaP9PhBaOGTjkni0nrv90hSHBHeu
ayM76vl2tzAB2KpDxHIHbI2SEdytV28l/rxWrsbk2kG7s7QB6VKiThTatYevh1kg
6Ujgl2xNSntG6t6PWjQ65OMSq5Iiwe7RfOZkNmPRGP/rbzMbgWXB3iCR2gVQvsxA
ctYdDZdUYAeQ5g6xSKJBloygDQdLzk9m0hqNUyl6gxhRNRw69hqp9E4mxFF3L1Gi
Z2LspLX1ZZWMh0bp0AniWzZuzLuNI0KYhlok6fnshjidJ1qYM/lbz5eJTYWxoJZW
rczn8gB3+MZbULc/iiO5Ag0EYrVn/gEQALrFLQjCR3GjuHSindz0rd3Fnx/t7Sen
T+p07yCSSoSlmnJHCQmwh4vfg1blyz0zZ4vkIhtpHsEgc+ZAG+WQXSsJ2iRz+eSN
GwoOQl4XC3n+QWkc1ws+btr48+6UqXIQU+F8TPQyx/PIgi2nZXJB7f5+mjCqsk46
XvH4nTr4kJjuqMSR/++wvre2qNQRa/q/dTsK0OaN/mJsdX6Oi+aGNaQJUhIG7F+E
ZDMkn/O6xnwWNzy/+bpg43qH/Gk0eakOmz5NmQLRkV58SZLiJvuCUtkttf6CyhnX
03OcWaajv5W8qA39dBYQgDrrPbBWUnwfO3yMveqhwV4JjDoe8sPAyn1NwzakNYqP
RzsWyLrLS7R7J9s3FkZXhQw/QQcsaSMcGNQO047dm1P83N8JY5aEpiRo9zSWjoiw
qoExANj5lUTZPe8M50lI182FrcjAN7dClO3QI6pg7wy0erMxfFly3j8UQ91ysS9T
s+GsP9I3cmWWQcKYxWHtE8xTXnNCVPFZQj2nwhJzae8ypfOtulBRA3dUKWGKuDH/
axFENhUsT397aOU3qkP/od4a64JyNIEo4CTTSPVeWd7njsGqli2U3A4xL2CcyYvt
D/MWcMBGEoLSNTswwKdom4FaJpn5KThnK/T0bQcmJblJhoCtppXisbexZnCpuS0x
Zdlm2T14KJ3LABEBAAGJAjwEGAEIACYCGwwWIQQ94D0eFEp/BpNZmdyq/SOBb80t
WwUCamLcngUJCY6oIAAKCRCq/SOBb80tWxilD/41xF1pwD2lGqjWLmlXcd0Ok5xm
67TSGiKiFZUgI3ftkRVlqmeJqjVmRjpc2dLfCuVBzlaP0Oyv5Jn+2CPqgmWIk5tw
pqlxqmsXiIZ9Uv2wWItxrXIwQ6DHc1qcIOu7feTnt1HVhxp2GqlXl3tA/+MTTDmn
4rqrHffI7ps9eCbK/SfSuoQk3+x/x4GPqWDbsNw1E9dDjPPhsKfdwzZ6dFR2Yhzu
2U0GDc2aeBA0Gjy6VFBj1ksmGxQCL3TqtGEHOonoPpTIjmDd35Xq6BSu119onOQS
THZ7+3JQRpizBVrIklKf33UOikZy9WHuffFddP46m8qC2Alh1aB7xcD8K3P9gzd5
OJO9Zr8JR+rvi8eOsCHvJqGhe0JMMf9bgXKjQ1tMTDTe6r0Jnw7HbkDghjpj9xG7
GYmDB0CL4kP5nAZ1OPfIasIu3Llhcy6mS52ECw0X6PNwUINwBn7F6EKAiciw8/OS
CXoGJDI5alwNhh+3DL4oBH1aq1lnmxmhFNpwpRDvtbRGmZeeASTeC4LMAtqP85Er
8W9ulIgQ4Ub8QuoxvE5hzWEB1QUkJgozI8vO4DoGlrsz7P8cPzrOT7WqUfUPTYE6
oZEfEYDSj1IZHnPYPS+yLBJMJ/Lc1h8OCetmbJZx4QWaX3Fgar7Zks8FfJQOTN6i
HnOlrsaMDYNyO76uBQ==
=4kVJ
-----END PGP PUBLIC KEY BLOCK-----
```

Security reports are greatly appreciated and we will publicly thank you for it, although we keep your name confidential if you request it.
