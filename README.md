# encoding/msgpack

MessagePack encoding package for Go

## Usage

### Pack/Unpack
    packed := msgpack.Marshal([]interface{}{1, nil, "three"})

    var unpacked interface{}
    msgpack.Unmarshal(packed, &unpacked)

### Streaming
    enc := msgpack.NewEncoder(os.Stdout)

    enc.Encode(true)
    enc.Encode(false)
    enc.Encode([]int{1, 2, 3})

    dec := msgpack.NewDecoder(os.Stdin)

    for {
      var data interface{}
      
      if dec.Decode(&data) != nil {
        break
      }

      fmt.Println(data)
    }

### Struct

    type Person struct {
      Name string `msgpack:"name"`
      Age  uint8  `msgpack:"age"`
    }

    var unpacked Person
    packed := msgpack.Marshal(Person{"hinasssan", 16})
    msgpack.Unmarshal(packed, &unpacked)
    fmt.Println(unpacked.Name)
    
