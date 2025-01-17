package module

import (
	"fmt"
	"os"

	"github.com/kyma-project/cli/pkg/module/git"
	"github.com/mandelsoft/vfs/pkg/projectionfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"github.com/open-component-model/ocm/pkg/common/accessobj"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/attrs/compatattr"
	ocm "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	v1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/comparch"
)

// Build creates a component archive with the given configuration.
// An empty vfs.FileSystem causes a FileSystem to be created in
// the temporary OS folder
func Build(fs vfs.FileSystem, path string, def *Definition) (*comparch.ComponentArchive, error) {
	if err := def.validate(); err != nil {
		return nil, err
	}
	return build(fs, path, def)
}

func build(fs vfs.FileSystem, path string, def *Definition) (*comparch.ComponentArchive, error) {
	// build minimal archive

	if err := fs.MkdirAll(path, os.ModePerm); err != nil {
		return nil, fmt.Errorf("unable to create component-archive path %q: %w", fs.Normalize(path), err)
	}
	archiveFs, err := projectionfs.New(fs, path)
	if err != nil {
		return nil, fmt.Errorf("unable to create projectionfilesystem: %w", err)
	}

	ctx := cpi.DefaultContext()
	if err := compatattr.Set(ctx, def.SchemaVersion == compdescv2.SchemaVersion); err != nil {
		return nil, fmt.Errorf("could not set compatibility attribute for v2: %w", err)
	}

	archive, err := comparch.New(
		ctx,
		accessobj.ACC_CREATE, archiveFs,
		nil,
		nil,
		vfs.ModePerm,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to build archive for minimal descriptor: %w", err)
	}

	cd := archive.GetDescriptor()
	cd.Metadata.ConfiguredVersion = def.SchemaVersion
	builtByCLI, err := v1.NewLabel("kyma-project.io/built-by", "cli", v1.WithVersion("v1"))
	if err != nil {
		return nil, err
	}

	if compatattr.Get(ctx) {
		cd.Provider = v1.Provider{Name: "internal"}
	} else {
		cd.Provider = v1.Provider{Name: "kyma-project.io", Labels: v1.Labels{*builtByCLI}}
	}

	if err := addSources(ctx, cd, def); err != nil {
		return nil, err
	}
	cd.ComponentSpec.SetName(def.Name)
	cd.ComponentSpec.SetVersion(def.Version)

	ocm.DefaultResources(cd)

	if err := ocm.Validate(cd); err != nil {
		return nil, fmt.Errorf("unable to validate component descriptor: %w", err)
	}

	return archive, nil
}

func addSources(ctx cpi.Context, cd *ocm.ComponentDescriptor, def *Definition) error {
	src, err := git.Source(ctx, def.Source, def.Repo, def.Version)
	if err != nil {
		return err
	}

	if idx := cd.GetSourceIndex(&src.SourceMeta); idx < 0 {
		cd.Sources = append(cd.Sources, *src)
	} else {
		cd.Sources[idx] = *src
	}

	return nil
}
