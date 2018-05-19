# Govarint

This project aims to provide a simple API for the performant encoding and decoding of 32 and 64 bit integers using a variety of algorithms.

[![](http://i.imgur.com/mpgC23U.jpg)](https://www.flickr.com/photos/tsevis/8648521649/)

## Usage

Each integer encoding algorithm conforms to an encoding and decoding interface.
The interfaces also specify the size of the unsigned integer, either 32 or 64 bits, and will be referred to as XX below.
To create an encoder:

    NewU32Base128Encoder(w io.Writer)
    NewU64Base128Encoder(w io.Writer)
    NewU32GroupVarintEncoder(w io.Writer)

For encoders, the only two commands are `PutUXX` and `Close`.
`Close` must be called as some integer encoding algorithms write in multiples.

    var buf bytes.Buffer
    enc := NewU32Base128Encoder(&buf)
    enc.PutU32(117)
    enc.PutU32(343)
    enc.Close()

To create a decoder:

    NewU32Base128Decoder(r io.ByteReader)
    NewU64Base128Decoder(r io.ByteReader)
    NewU32GroupVarintDecoder(r io.ByteReader)

For decoders, the only command is `GetUXX`.
`GetUXX` returns the value and any potential errors.
When reading is complete, `GetUXX` will return an `EOF` (End Of File).

    dec := NewU32Base128Decoder(&buf)
    x, err := dec.GetU32()

## Use Cases

Using fixed width integers, such as uint32 and uint64, usually waste large amounts of space, especially when encoding small values.
Optimally, smaller numbers should take less space to represent.

Using integer encoding algorithms is especially common in specific applications, such as storing edge lists or indexes for search engines.
In these situations, you have a sorted list of numbers that you want to keep as compactly as possible in memory.
Additionally, by storing only the difference between the given number and the previous (delta encoding), the numbers are quite small, and thus compress well.

For an explicit example, the Web Data Commons Hyperlink Graph contains 128 billion edges linking page A to page B, where each page is represented by a 32 bit integer.
By converting all these edges to 64 bit integers (32 | 32), sorting them, and then using delta encoding, memory usage can be reduced from 64 bits per edge down to only 9 bits per edge using the Base128 integer encoding algorithm.
This figure improves even further if compressed using conventional compression algorithms (3 bits per edge).

## Encodings supported

`govarint` supports:

+ Base128 [32, 64] - each byte uses 7 bits for encoding the integer and 1 bit for indicating if the integer requires another byte
+ Group Varint [32] - integers are encoded in blocks of four - one byte encodes the size of the following four integers, then the values of the four integers follows

Group Varint consistently beats Base128 in decompression speed but Base128 may offer improved compression ratios depending on the distribution of the supplied integers.

## Tests

    go test -v -bench=.

## License

MIT License, as per `LICENSE`
