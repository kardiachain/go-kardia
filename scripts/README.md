# Install protobuf & dependencies
```
go install github.com/gogo/protobuf/proto
go install github.com/gogo/protobuf/protoc-gen-gogofaster
go install github.com/gogo/protobuf/gogoproto
```

# Make the script executable
```
# Assume you're at go-kardia directory
chmod +x ./scripts/protocgen.sh
```

# Generate Go bindings for protobuf
```
# Assume you're at go-kardia directory
./scripts/protocgen.sh
```