package logic

import (
	"brick/log"
	"fmt"
	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/emicklei/proto"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type ErrCodeDef struct {
	ErrCodeSetName string
	ErrCodeEnums   []*proto.EnumField
}

type ImportNode struct {
	ImportPath string
	GoPackage  string
}

type RpcNode struct {
	MethodName string
	ReqType    string
	RspType    string
	CmdID      string
	Url        string
	Flags      string

	CommentLines []string
	commentMap   map[string]*linesCommentNode
}

type ProtoDetect struct {
	PackageName   string
	SvrName       string
	GoPackageName string
	RpcList       []*RpcNode
	ErrCodes      []ErrCodeDef

	SvrDef map[string]string

	FuncSvr   map[string]int
	FuncCli   map[string]int
	FuncLogic map[string]int
	FuncTool  map[string]int

	ImportList []*ImportNode
	IncPaths   []string
}

type PbField struct {
	NormalField *proto.NormalField
	MapField    *proto.MapField
	Msg         *PbMsg

	comment map[string]*linesCommentNode
}

func (p *PbField) GetName() string {
	if p.NormalField != nil {
		return p.NormalField.Name
	} else if p.MapField != nil {
		return p.MapField.Name
	}

	return ""
}

func (p *PbField) GetType() string {
	if p.NormalField != nil {
		return p.NormalField.Type
	} else if p.MapField != nil {
		return p.MapField.Type
	}

	return ""
}

func (p *PbField) GetPbComment() *proto.Comment {
	var c *proto.Comment
	if p.NormalField != nil {
		c = p.NormalField.Comment
	} else if p.MapField != nil {
		c = p.MapField.Comment
	}

	return c
}

type PbMsg struct {
	Name    string
	Fields  []*PbField
	ModName string

	NameDupCnt int
}

var PbMap = make(map[string]*PbMsg)

type ErrCodes struct {
	m map[string]map[string]uint32
}

func (p *ErrCodes) Set(mod string, key string, val uint32) {
	x := p.m[mod]
	if x == nil {
		x = make(map[string]uint32)
		p.m[mod] = x
	}

	x[key] = val
}

func (p *ErrCodes) Get(mod string, key string) uint32 {
	x := p.m[mod]
	if x == nil {
		return 0
	}

	v := x[key]
	return v
}

func (p *ErrCodes) GetAutoLoad(mod string, key string) uint32 {
	x := p.m[mod]
	if x == nil {
		x = make(map[string]uint32)
		p.m[mod] = x

		err := ParseErrCode(mod + ".proto")
		if err != nil {
			log.Fatalf("parse proto %s.proto err %s", mod, err)
		}
	}

	v := x[key]
	return v
}

var AllErrCodes = &ErrCodes{m: make(map[string]map[string]uint32)}

func NewProtoDetect() *ProtoDetect {
	var pd ProtoDetect

	pd.FuncCli = make(map[string]int)
	pd.FuncSvr = make(map[string]int)
	pd.FuncLogic = make(map[string]int)
	pd.SvrDef = make(map[string]string)
	pd.FuncTool = make(map[string]int)

	return &pd
}

func (pd *ProtoDetect) GetImportPbList() []string {
	var list []string
	for _, i := range pd.ImportList {
		list = append(list, i.GoPackage)
	}
	return list
}

type ServerComposement struct {
	mainL   token.Pos
	mainR   token.Pos
	imports []string
}

var ModName string
var CurrentPb string
var CurrentMod string
var PbIncPaths []string

func SetCurrentPb(pb string) {
	CurrentPb = pb
	p := strings.LastIndex(pb, "/")
	if p > 0 {
		pb = pb[p+1:]
	}

	if strings.HasSuffix(pb, ".proto") {
		pb = pb[:len(pb)-6]
	}

	CurrentMod = pb
}

func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

func GetTargetFileName(PD ProtoDetect, objType string, rootDir string) string {
	var fn, dirName string

	switch objType {
	case "def":
		fallthrough
	case "client":
		fallthrough
	case "errcode":
		fallthrough
	case "console":
		dirName = fmt.Sprintf("%s/", rootDir)

	case "conf":
		fallthrough
	case "supervisor_conf":
		fallthrough
	case "server":
		dirName = fmt.Sprintf("%s/server/", rootDir)

	case "logic_state_obj_cache":
		fallthrough
	case "logic_state_redis":
		fallthrough
	case "logic_state_db":
		fallthrough
	case "logic_cfg":
		fallthrough
	case "logic":
		dirName = fmt.Sprintf("%s/impl/", rootDir)

	case "tool":
		dirName = fmt.Sprintf("%s/tool/", rootDir)

	default:
		log.Fatal("unknown obj type", objType)
	}

	CheckPrepareDir(dirName)

	switch objType {
	case "conf":
		fn = fmt.Sprintf("%s%s.toml", dirName, PD.SvrName)
	case "def":
		fn = fmt.Sprintf("%s%sdef.go", dirName, PD.SvrName)
	case "client":
		fn = fmt.Sprintf("%s%sclient.go", dirName, PD.SvrName)
	case "errcode":
		fn = fmt.Sprintf("%s%serrcode.go", dirName, PD.SvrName)
	case "server":
		fn = fmt.Sprintf("%s%s.go", dirName, PD.SvrName)
	case "logic":
		fn = fmt.Sprintf("%s%simpl.go", dirName, PD.SvrName)
	case "logic_cfg":
		fn = fmt.Sprintf("%s%scfg.go", dirName, PD.SvrName)
	case "logic_state_db":
		fn = fmt.Sprintf("%s%sstatedb_autogen.go", dirName, PD.SvrName)
	case "logic_state_redis":
		fn = fmt.Sprintf("%s%sstateredis_autogen.go", dirName, PD.SvrName)
	case "logic_state_obj_cache":
		fn = fmt.Sprintf("%s%sstateobjcache_autogen.go", dirName, PD.SvrName)
	case "supervisor_conf":
		fn = fmt.Sprintf("%ssupervisor.%s.conf", dirName, PD.SvrName)
	case "tool":
		fn = fmt.Sprintf("%s%s_tool.go", dirName, PD.SvrName)
	}

	return fn
}

func ParseGoCode(PD *ProtoDetect, fn string, objType string) {
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, fn, nil, 0)
	if err != nil {
		log.Fatalf("parse file error %v %v %v", objType, err, fn)
	}

	for _, decl := range f.Decls {
		switch t := decl.(type) {
		case *ast.FuncDecl:
			if objType == "client" {
				PD.FuncCli[t.Name.Name] = 1
			} else if objType == "logic" {
				PD.FuncLogic[t.Name.Name] = 1
			} else if objType == "server" {
				PD.FuncSvr[t.Name.Name] = 1
			} else if objType == "tool" {
				PD.FuncTool[t.Name.Name] = 1
			}
		case *ast.GenDecl:
			if objType == "def" {
				for _, spec := range t.Specs {
					switch spec := spec.(type) {
					case *ast.ImportSpec:
						fmt.Println("Import", spec.Path.Value)
					case *ast.TypeSpec:
						fmt.Println("Type", spec.Name.String())
					case *ast.ValueSpec:
						for _, id := range spec.Names {
							PD.SvrDef[id.Name] = id.Obj.Decl.(*ast.ValueSpec).Values[0].(*ast.BasicLit).Value
						}
					}
				}
			}
		}
	}
}

func ParseGoServer(s *ServerComposement, fn string) {
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, fn, nil, 0)
	if err != nil {
		fmt.Printf("Parse  error : %v\n", err)
		return
	}
	if s.imports == nil {
		s.imports = make([]string, 0)
	}
	for _, decl := range f.Decls {
		switch t := decl.(type) {
		// That's a func decl !
		case *ast.FuncDecl:
			if t.Name.Name == "main" {
				s.mainL = t.Body.Lbrace
				s.mainR = t.Body.Rbrace
			}
		case *ast.GenDecl:
			for _, spec := range t.Specs {
				switch v := spec.(type) {
				case *ast.ImportSpec:
					s.imports = append(s.imports, v.Path.Value)
				case *ast.ValueSpec:
				case *ast.TypeSpec:
				default:
					log.Info(v)
				}
			}

		default:
			fmt.Printf("decl type,%v", t)

			// Now I would like to access to the test var and get its type
			// Must be int, because foo() returns an int
		}
	}
}

func FileExists(name string) bool {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func CheckPrepareDir(dir string) {
	stat, err := os.Stat(dir)
	if err != nil || !stat.IsDir() {
		err := os.MkdirAll(dir, 0744)
		if err != nil {
			log.Fatal("make dir error", dir, err)
		}
	}
	return
}

func JoinImportList(list []string) string {
	var transList []string
	for _, i := range list {
		if strings.LastIndex(i, "\"") >= 0 {
			transList = append(transList, fmt.Sprintf("\t%s", i))
		} else {
			transList = append(transList, fmt.Sprintf("\t\"%s\"", i))
		}
	}
	return strings.Join(transList, "\n")
}

func JoinImportListWithBuf(list []string, buf string) string {
	// 过滤掉不需要 import 的头
	var impListFiltered []string
	for _, x := range list {
		i := strings.LastIndex(x, "/")
		j := strings.Index(x, " ")
		t := x
		if j > 0 {
			t = x[:j]
		} else if i >= 0 {
			t = x[i+1:]
		}
		t += "."
		if strings.Index(buf, t) >= 0 || x == "pinfire/perm" {
			impListFiltered = append(impListFiltered, x)
		}
	}
	return JoinImportList(impListFiltered)
}

func FindServerRegisterFile() string {
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Error(err)
		return ""
	}

	for abs != "" {
		p := path.Join(abs, "server_register.txt")
		_, err := os.Stat(p)
		if err == nil {
			return p
		}

		prev := abs
		abs = filepath.Dir(abs)
		if prev == abs {
			break
		}
	}
	return ""
}

var sep string

func init() {
	if runtime.GOOS == "windows" {
		sep = `\`
	} else {
		sep = "/"
	}
}

func SearchImportPb(impPath string) string {
	if fileutil.Exist(impPath) {
		return impPath
	}

	for _, incPath := range PbIncPaths {
		p := fmt.Sprintf("%s%s%s", incPath, sep, impPath)
		if fileutil.Exist(p) {
			return p
		}
	}

	return ""
}

func ParseErrCode(protoFile string) error {
	full := SearchImportPb(protoFile)
	if full == "" {
		err := fmt.Errorf("not found %s", protoFile)
		log.Error(err)
		return err
	}

	reader, err := os.Open(full)
	if err != nil {
		log.Errorf("can not open proto file %s, error is %v", protoFile, err)
		return err
	}
	defer reader.Close()

	p := proto.NewParser(reader)
	definition, err := p.Parse()
	if err != nil {
		log.Fatalf("proto parse error %v", err)
	}

	old := CurrentPb
	SetCurrentPb(protoFile)
	defer SetCurrentPb(old)

	handleEnum := func(e *proto.Enum) {
		if strings.HasSuffix(e.Name, "ErrCode") {
			var pv ProtoVisitor
			for _, ei := range e.Elements {
				ei.Accept(&pv)
			}
		}
	}

	proto.Walk(
		definition,
		proto.WithEnum(handleEnum),
	)

	return nil
}
