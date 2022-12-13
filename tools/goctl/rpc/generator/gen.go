package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeromicro/go-zero/tools/goctl/rpc/execx"
	"github.com/zeromicro/go-zero/tools/goctl/rpc/generator/ent"
	"github.com/zeromicro/go-zero/tools/goctl/rpc/parser"
	"github.com/zeromicro/go-zero/tools/goctl/util/console"
	"github.com/zeromicro/go-zero/tools/goctl/util/ctx"
	"github.com/zeromicro/go-zero/tools/goctl/util/pathx"
)

type ZRpcContext struct {
	// Sre is the source file of the proto.
	Src string
	// ProtoCmd is the command to generate proto files.
	ProtocCmd string
	// ProtoGenGrpcDir is the directory to store the generated proto files.
	ProtoGenGrpcDir string
	// ProtoGenGoDir is the directory to store the generated go files.
	ProtoGenGoDir string
	// IsGooglePlugin is the flag to indicate whether the proto file is generated by google plugin.
	IsGooglePlugin bool
	// GoOutput is the output directory of the generated go files.
	GoOutput string
	// GrpcOutput is the output directory of the generated grpc files.
	GrpcOutput string
	// Output is the output directory of the generated files.
	Output string
	// Multiple is the flag to indicate whether the proto file is generated in multiple mode.
	Multiple bool
	// Schema is the ent schema path
	Schema string
	// Ent
	Ent bool
	// ModuleName is the module name in go mod
	ModuleName string
	// GoZeroVersion describes the version of Go Zero
	GoZeroVersion string
	// ToolVersion describes the version of Simple Admin Tools
	ToolVersion string
	// Port describes the service port exposed
	Port int
	// MakeFile describes whether generate makefile
	MakeFile bool
	// DockerFile describes whether generate dockerfile
	DockerFile bool
}

// Generate generates a rpc service, through the proto file,
// code storage directory, and proto import parameters to control
// the source file and target location of the rpc service that needs to be generated
func (g *Generator) Generate(zctx *ZRpcContext) error {
	abs, err := filepath.Abs(zctx.Output)
	if err != nil {
		return err
	}

	err = pathx.MkdirIfNotExist(abs)
	if err != nil {
		return err
	}

	err = g.Prepare()
	if err != nil {
		return err
	}

	if zctx.ModuleName != "" {
		_, err = execx.Run("go mod init "+zctx.ModuleName, abs)
		if err != nil {
			return err
		}
	}

	if zctx.GoZeroVersion != "" && zctx.ToolVersion != "" {
		_, err := execx.Run(fmt.Sprintf("goctls migrate --zero-version %s --tool-version %s", zctx.GoZeroVersion, zctx.ToolVersion),
			abs)
		if err != nil {
			return err
		}
	}

	projectCtx, err := ctx.Prepare(abs)
	if err != nil {
		return err
	}

	p := parser.NewDefaultProtoParser()
	proto, err := p.Parse(zctx.Src, zctx.Multiple)
	if err != nil {
		return err
	}

	dirCtx, err := mkdir(projectCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenEtc(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenPb(dirCtx, zctx)
	if err != nil {
		return err
	}

	err = g.GenConfig(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenSvc(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenLogic(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenServer(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenMain(dirCtx, proto, g.cfg, zctx)
	if err != nil {
		return err
	}

	err = g.GenCall(dirCtx, proto, g.cfg, zctx)

	if zctx.MakeFile {
		err = g.GenMakefile(dirCtx, proto, g.cfg, zctx)
		if err != nil {
			return err
		}
	}

	if zctx.DockerFile {
		err = g.GenDockerfile(dirCtx, proto, g.cfg, zctx)
		if err != nil {
			return err
		}
	}

	// generate ent
	if zctx.Ent {
		_, err := execx.Run(fmt.Sprintf("go run -mod=mod entgo.io/ent/cmd/ent init %s",
			dirCtx.GetServiceName().ToCamel()), abs)
		if err != nil {
			return err
		}

		_, err = execx.Run("go mod tidy", abs)
		if err != nil {
			return err
		}

		_, err = execx.Run("go run -mod=mod entgo.io/ent/cmd/ent generate ./ent/schema", abs)
		if err != nil {
			return err
		}

		err = pathx.MkdirIfNotExist(filepath.Join(abs, "ent", "template"))
		if err != nil {
			return err
		}

		paginationTplPath := filepath.Join(abs, "ent", "template", "pagination.tmpl")
		if !pathx.FileExists(paginationTplPath) {
			err = os.WriteFile(paginationTplPath, []byte(ent.PaginationTpl), os.ModePerm)
			if err != nil {
				return err
			}
		}

		_, err = execx.Run("make gen-rpc", abs)
		if err != nil {
			return err
		}
	}

	console.NewColorConsole().MarkDone()

	return err
}
