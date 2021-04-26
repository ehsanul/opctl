package model

// AddAuthReq holds data for adding source (git or OCI Distribution API) credentials
type AddAuthReq struct {
	// Resources designates which resources this auth is for in the form of a reference (or prefix of).
	Resources string
	Creds
}

// GetDataReq deprecated
type GetDataReq struct {
	ContentPath string
	PkgRef      string
}

// ListDescendantsReq deprecated
type ListDescendantsReq struct {
	PkgRef string `json:"pkgRef"`
}

type StartOpReq struct {
	// map of args keyed by input name
	Args map[string]*Value `json:"args,omitempty"`
	// Op details the op to start
	Op StartOpReqOp `json:"op,omitempty"`
}

type StartOpReqOp struct {
	Ref string
}
