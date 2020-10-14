package service

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"log"
	"os"
	"path"

	"k8s.io/helm/cmd/helm/search"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/repo"
)

const (
	downloadedFileName = "downloaded-index.yaml"
	indexFileName      = "index.yaml"
)

// GetServiceInterface defines a Get service
type GetServiceInterface interface {
	Get() error
}

// GetService structure definition
type GetService struct {
	config       repo.Entry
	verbose      bool
	ignoreErrors bool
	logger       *log.Logger
	newRootURL   string
	allVersions  bool
	chartName    string
	chartVersion string
}

// NewGetService return a new instace of GetService
func NewGetService(config repo.Entry, allVersions bool, verbose bool, ignoreErrors bool, logger *log.Logger, newRootURL string, chartName string, chartVersion string) GetServiceInterface {
	return &GetService{
		config:       config,
		verbose:      verbose,
		ignoreErrors: ignoreErrors,
		logger:       logger,
		newRootURL:   newRootURL,
		allVersions:  allVersions,
		chartName:    chartName,
		chartVersion: chartVersion,
	}
}

//Get methods downloads the index file and the Helm charts to the working directory.
func (g *GetService) Get() error {
	chartRepo, err := repo.NewChartRepository(&g.config, getter.All(environment.EnvSettings{}))
	if err != nil {
		return err
	}

	downloadedIndexPath := path.Join(g.config.Name, downloadedFileName)
	err = chartRepo.DownloadIndexFile(downloadedIndexPath)
	if err != nil {
		return err
	}

	err = chartRepo.Load()
	if err != nil {
		return err
	}

	chartPrefix := ""
	chartPath := ""
	index := search.NewIndex()
	index.AddRepo(chartRepo.Config.Name, chartRepo.IndexFile, (g.allVersions || g.chartVersion != ""))
	rexp := fmt.Sprintf("^.*%s.*", g.chartName)
	res, err := index.Search(rexp, 1, true)
	if err != nil {
		return err
	}

	for _, r := range res {
		if g.chartName != "" && r.Chart.Name != g.chartName {
			continue
		}
		if g.chartVersion != "" && r.Chart.Version != g.chartVersion {
			continue
		}
		for _, u := range r.Chart.URLs {
			urlParsed, _ := url.Parse(u)
			chartPrefix, _ = path.Split(urlParsed.Path)

			b, err := chartRepo.Client.Get(u)
			if err != nil {
				if g.ignoreErrors {
					g.logger.Printf("WARNING: processing chart %s(%s) - %s", r.Name, r.Chart.Version, err)
					continue
				} else {
					return err
				}
			}
			chartFileName := fmt.Sprintf("%s-%s.tgz", r.Chart.Name, r.Chart.Version)
			if chartPrefix != "" {
				chartPath = path.Join(g.config.Name, chartPrefix, chartFileName)
			} else {
				chartPath = path.Join(g.config.Name, chartFileName)
			}
			err = writeFile(chartPath, b.Bytes(), g.logger, g.ignoreErrors)
			if err != nil {
				return err
			}
		}
	}

	err = prepareIndexFile(g.config.Name, g.config.URL, g.newRootURL, g.logger, g.ignoreErrors)
	if err != nil {
		return err
	}
	return nil
}

func writeFile(name string, content []byte, log *log.Logger, ignoreErrors bool) error {
	// Create required subfolders structure
	err := os.MkdirAll(path.Dir(name), 0744)
	if err != nil {
	  if ignoreErrors {
		  log.Printf("cannot create destination folder: %s", name, err)
	  } else {
		  return err
	  }
	}

	// Write destination file
	err = ioutil.WriteFile(name, content, 0666)
	if err != nil {
	  if ignoreErrors {
		  log.Printf("cannot write files %s: %s", name, err)
	  } else {
		  return err
	  }
	}
	return nil
}

func prepareIndexFile(folder string, repoURL string, newRootURL string, log *log.Logger, ignoreErrors bool) error {
	downloadedPath := path.Join(folder, downloadedFileName)
	indexPath := path.Join(folder, indexFileName)
	if newRootURL != "" {
		indexContent, err := ioutil.ReadFile(downloadedPath)
		if err != nil {
			return err
		}
		content := bytes.Replace(indexContent, []byte(repoURL), []byte(newRootURL), -1)
		err = writeFile(downloadedPath, []byte(content), log, ignoreErrors)
		if err != nil {
			return nil
		}
	}
	return os.Rename(downloadedPath, indexPath)
}
