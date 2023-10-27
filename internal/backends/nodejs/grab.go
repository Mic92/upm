package nodejs

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/replit/upm/internal/api"
	"github.com/replit/upm/internal/util"
	"github.com/smacker/go-tree-sitter/javascript"
)

var internalModules = []string{
	"assert",
	"async_hooks",
	"buffer",
	"child_process",
	"cluster",
	"console",
	"constants",
	"crypto",
	"dgram",
	"dns",
	"domain",
	"events",
	"fs",
	"http",
	"http2",
	"https",
	"inspector",
	"module",
	"net",
	"os",
	"path",
	"perf_hooks",
	"process",
	"punycode",
	"querystring",
	"readline",
	"repl",
	"stream",
	"string_decoder",
	"sys",
	"timers",
	"tls",
	"trace_events",
	"tty",
	"url",
	"util",
	"v8",
	"vm",
	"worker_threads",
	"zlib",
}

type parseResult struct {
	importPaths []string
	ok          bool
}

func parseFile(path string, results chan parseResult) {
	language := javascript.GetLanguage()

	// NOTE: only `@import` tags are handled.
	importsQuery := `
(import_statement
  source: (string) @import)

((call_expression
     function: [(identifier) @function
		            (import)]
     arguments: (arguments . (string) @import .))
 (#eq? @function "require"))
`

	importPaths, err := util.QueryImportsFromTreeSitter(path, language, importsQuery)
	if err != nil {
		log.Println("failed to guess imports from", path, ":", err)
	}

	results <- parseResult{
		importPaths: importPaths,
		ok:          err == nil,
	}
}

func guessBareImports() map[api.PkgName]bool {
	pkgs := map[api.PkgName]bool{}
	results := make(chan parseResult)
	numParsedFiles := 0
	var visitDir func(dirName string)

	visitDir = func(dirName string) {
		for _, ignoredPath := range util.IgnoredPaths {
			if ignoredPath == filepath.Base(dirName) {
				return
			}
		}

		files, err := os.ReadDir(dirName)
		if err != nil {
			log.Fatalln(err)
		}

		for _, file := range files {
			absPath := filepath.Join(dirName, file.Name())
			if file.IsDir() {
				visitDir(absPath)
				continue
			}

			extension := path.Ext(absPath)
			if extension != ".js" &&
				extension != ".jsx" &&
				extension != ".tsx" &&
				extension != ".ts" &&
				extension != ".mjs" &&
				extension != ".cjs" {
				continue
			}

			numParsedFiles++
			go parseFile(absPath, results)
		}
	}

	dir, err := filepath.Abs(".")
	if err != nil {
		log.Fatalln(err)
	}

	visitDir(dir)

	for i := 0; i < numParsedFiles; i++ {
		result := <-results
		if !result.ok {
			continue
		}

		for _, mod := range result.importPaths {
			// Since Node.js 16, you can prefix the import path with `node:` to denote that the
			// module is a core module.
			if strings.HasPrefix(mod, "node:") {
				continue
			}

			// Skip empty imports
			if mod == "" {
				continue
			}

			// Skip absolute imports
			if mod[0] == '/' {
				continue
			}

			// Skip relative imports
			if mod[0] == '.' {
				continue
			}

			// Skip external files, don't import from http or https
			if strings.HasPrefix(mod, "http:") || strings.HasPrefix(mod, "https:") {
				continue
			}

			// Skip script loaders
			if strings.Contains(mod, "!") {
				continue
			}

			// Handle scoped modules or internal modules
			if mod[0] == '@' {
				parts := strings.Split(mod, "/")
				if len(parts) < 2 {
					continue
				}
				mod = strings.Join(parts[:2], "/")
			} else {
				parts := strings.Split(mod, "/")
				mod = parts[0]

				isInternalMod := false
				for _, internal := range internalModules {
					if internal == mod {
						isInternalMod = true
						break
					}
				}
				if isInternalMod {
					continue
				}
			}

			pkgs[api.PkgName(mod)] = true
		}
	}

	return pkgs
}
