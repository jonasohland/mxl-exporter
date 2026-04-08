package mxl

type rawCommonFlowConfigInfo struct {
	ID                     [16]byte
	Format                 uint32
	Flags                  uint32
	Rate                   Rational
	MaxCommitBatchSizeHint uint32
	MaxSyncBatchSizeHint   uint32
	PayloadLocation        uint32
	DeviceIndex            uint32
	Reserved               [72]byte
}

type rawContFlowConfigInfo struct {
	Channels     uint32
	BufferLength uint32
	Reserved     [56]byte
}

type rawDiscreteFlowConfigInfo struct {
	SliceSizes [4]uint32
	GrainCount uint32
	Reserved   [44]byte
}

type rawFlowConfigInfo struct {
	Common   rawCommonFlowConfigInfo
	Concrete [64]byte
}

type rawFlowRuntimeInfo struct {
	HeadIndex     uint64
	LastWriteTime uint64
	LastReadTime  uint64
	Reserved      [40]byte
}

type rawFlowInfo struct {
	Version  uint32
	Size     uint32
	Config   rawFlowConfigInfo
	Runtime  rawFlowRuntimeInfo
	Reserved [1784]byte
}
