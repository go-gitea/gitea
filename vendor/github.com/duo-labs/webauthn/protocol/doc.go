// The protocol package contains data structures and validation functionality
// outlined in the Web Authnentication specification (https://www.w3.org/TR/webauthn).
// The data structures here attempt to conform as much as possible to their definitions,
// but some structs (like those that are used as part of validation steps) contain
// additional fields that help us unpack and validate the data we unmarshall.
// When implementing this library, most developers will primarily be using the API
// outlined in the webauthn package.
package protocol
