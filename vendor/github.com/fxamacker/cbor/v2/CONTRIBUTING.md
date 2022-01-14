# How to contribute

This project started because I needed an easy, small, and crash-proof CBOR library for my [WebAuthn (FIDO2) server library](https://github.com/fxamacker/webauthn). I believe this was the first and still only standalone CBOR library (in Go) that is fuzz tested as of November 10, 2019.

To my surprise, Stefan Tatschner (rumpelsepp) submitted the first 2 issues when I didn't expect this project to be noticed.  So I decided to make it more full-featured for others by announcing releases and asking for feedback. Even this document exists because Montgomery Edwards⁴⁴⁸ (x448) opened [issue #22](https://github.com/fxamacker/cbor/issues/22).  In other words, you can contribute by opening an issue that helps the project improve. Especially in the early stages.

When I announced v1.2 on Go Forum, Jakob Borg (calmh) responded with a thumbs up and encouragement.  Another project of equal priority needed my time and Jakob's kind words tipped the scale for me to work on this one (speedups for [milestone v1.3](https://github.com/fxamacker/cbor/issues?q=is%3Aopen+is%3Aissue+milestone%3Av1.3.0).) So words of appreciation or encouragement is nice way to contribute to open source projects.

Another way is by using this library in your project. It can lead to features that benefit both projects, which is what happened when oasislabs/oasis-core switched to this CBOR libary -- thanks Yawning Angel (yawning) for requesting BinaryMarshaler/BinaryUnmarshaler and Jernej Kos (kostco) for requesting RawMessage!

If you'd like to contribute code or send CBOR data, please read on (it can save you time!)

## Private reports
Usually, all issues are tracked publicly on [GitHub](https://github.com/fxamacker/cbor/issues). 

To report security vulnerabilities, please email faye.github@gmail.com and allow time for the problem to be resolved before disclosing it to the public.  For more info, see [Security Policy](https://github.com/fxamacker/cbor#security-policy).

Please do not send data that might contain personally identifiable information, even if you think you have permission.  That type of support requires payment and a contract where I'm indemnified, held harmless, and defended for any data you send to me.

## Prerequisites to pull requests
Please [create an issue](https://github.com/fxamacker/cbor/issues/new/choose), if one doesn't already exist, and describe your concern. You'll need a [GitHub account](https://github.com/signup/free) to do this.

If you submit a pull request without creating an issue and getting a response, you risk having your work unused because the bugfix or feature was already done by others and being reviewed before reaching Github.

## Describe your issue
Clearly describe the issue:
* If it's a bug, please provide: **version of this library** and **Go** (`go version`), **unmodified error message**, and describe **how to reproduce it**.  Also state **what you expected to happen** instead of the error.
* If you propose a change or addition, try to give an example how the improved code could look like or how to use it.
* If you found a compilation error, please confirm you're using a supported version of Go. If you are, then provide the output of `go version` first, followed by the complete error message.

## Please don't
Please don't send data containing personally identifiable information, even if you think you have permission.  That type of support requires payment and a contract where I'm indemnified, held harmless, and defended for any data you send to me.

Please don't send CBOR data larger than 512 bytes. If you want to send crash-producing CBOR data > 512 bytes, please get my permission before sending it to me.

## Wanted
* Opening issues that are helpful to the project
* Using this library in your project and letting me know
* Sending well-formed CBOR data (<= 512 bytes) that causes crashes (none found yet).
* Sending malformed CBOR data (<= 512 bytes) that causes crashes (none found yet, but bad actors are better than me at breaking things).
* Sending tests or data for unit tests that increase code coverage (currently at 97.8% for v1.2.)
* Pull requests with small changes that are well-documented and easily understandable.
* Sponsors, donations, bounties, subscriptions: I'd like to run uninterrupted fuzzing between releases on a server with dedicated CPUs (after v1.3 or v1.4.)

## Credits
This guide used nlohmann/json contribution guidelines for inspiration as suggested in issue #22.

