package consensus

import "hash/crc32"

var crc32c = crc32.MakeTable(crc32.Castagnoli)
