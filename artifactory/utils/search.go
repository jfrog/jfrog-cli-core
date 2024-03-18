package utils

import (
	"encoding/json"
	"errors"
	"github.com/jfrog/jfrog-client-go/artifactory"

	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/content"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type SearchResult struct {
	Path         string              `json:"path,omitempty"`
	Type         string              `json:"type,omitempty"`
	Size         int64               `json:"size,omitempty"`
	Created      string              `json:"created,omitempty"`
	Modified     string              `json:"modified,omitempty"`
	Sha1         string              `json:"sha1,omitempty"`
	Sha256       string              `json:"sha256,omitempty"`
	Md5          string              `json:"md5,omitempty"`
	OriginalMd5  string              `json:"original_md5,omitempty"`
	ModifiedBy   string              `json:"modified_by,omitempty"`
	Updated      string              `json:"updated,omitempty"`
	CreatedBy    string              `json:"created_by,omitempty"`
	OriginalSha1 string              `json:"original_sha1,omitempty"`
	Depth        int                 `json:"depth,omitempty"`
	Props        map[string][]string `json:"props,omitempty"`
}

func PrintSearchResults(reader *content.ContentReader) error {
	length, err := reader.Length()
	if length == 0 {
		log.Output("[]")
		return err
	}
	log.Output("[")
	suffix := ","
	for searchResult := new(SearchResult); reader.NextRecord(searchResult) == nil; searchResult = new(SearchResult) {
		if length == 1 {
			suffix = ""
		}
		err = printSearchResult(*searchResult, suffix)
		if length == 0 {
			log.Output("[]")
			return err
		}
		length--
	}
	log.Output("]")
	reader.Reset()
	return reader.GetError()
}

func printSearchResult(toPrint SearchResult, suffix string) error {
	data, err := json.Marshal(toPrint)
	if err != nil {
		return errorutils.CheckError(err)
	}
	log.Output("  " + clientutils.IndentJsonArray(data) + suffix)
	return nil
}

func AqlResultToSearchResult(readers []*content.ContentReader) (contentReader *content.ContentReader, err error) {
	writer, err := content.NewContentWriter("results", true, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		e := writer.Close()
		if err == nil {
			err = e
		}
	}()
	for _, reader := range readers {
		for searchResult := new(utils.ResultItem); reader.NextRecord(searchResult) == nil; searchResult = new(utils.ResultItem) {
			if err != nil {
				return nil, err
			}
			tempResult := new(SearchResult)
			tempResult.Path = searchResult.Repo + "/"
			if searchResult.Path != "." {
				tempResult.Path += searchResult.Path + "/"
			}
			if searchResult.Name != "." {
				tempResult.Path += searchResult.Name
			}
			tempResult.Type = searchResult.Type
			tempResult.Size = searchResult.Size
			tempResult.Created = searchResult.Created
			tempResult.Modified = searchResult.Modified
			tempResult.Sha1 = searchResult.Actual_Sha1
			tempResult.Sha256 = searchResult.Sha256
			tempResult.Md5 = searchResult.Actual_Md5
			tempResult.ModifiedBy = searchResult.ModifiedBy
			tempResult.Updated = searchResult.Updated
			tempResult.CreatedBy = searchResult.CreatedBy
			tempResult.OriginalMd5 = searchResult.OriginalMd5
			tempResult.Depth = searchResult.Depth
			tempResult.Props = make(map[string][]string, len(searchResult.Properties))
			for _, prop := range searchResult.Properties {
				tempResult.Props[prop.Key] = append(tempResult.Props[prop.Key], prop.Value)
			}
			writer.Write(*tempResult)
		}
		if err = reader.GetError(); err != nil {
			return nil, err
		}
		reader.Reset()
	}
	contentReader = content.NewContentReader(writer.GetFilePath(), content.DefaultKey)
	return
}

func GetSearchParams(f *spec.File) (searchParams services.SearchParams, err error) {
	searchParams = services.NewSearchParams()
	searchParams.CommonParams, err = f.ToCommonParams()
	searchParams.Include = f.GetInclude()
	if err != nil {
		return
	}
	searchParams.Recursive, err = f.IsRecursive(true)
	if err != nil {
		return
	}
	searchParams.ExcludeArtifacts, err = f.IsExcludeArtifacts(false)
	if err != nil {
		return
	}
	searchParams.IncludeDeps, err = f.IsIncludeDeps(false)
	if err != nil {
		return
	}
	searchParams.IncludeDirs, err = f.IsIncludeDirs(false)
	if err != nil {
		return
	}
	searchParams.Transitive, err = f.IsTransitive(false)
	searchParams.Include = f.GetInclude()
	return
}

func SearchResultNoDate(reader *content.ContentReader) (contentReader *content.ContentReader, err error) {
	writer, err := content.NewContentWriter("results", true, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		e := writer.Close()
		if err == nil {
			err = e
		}
	}()
	for resultItem := new(SearchResult); reader.NextRecord(resultItem) == nil; resultItem = new(SearchResult) {
		if err != nil {
			return nil, err
		}
		resultItem.Created = ""
		resultItem.Modified = ""
		delete(resultItem.Props, "vcs.url")
		delete(resultItem.Props, "vcs.revision")
		writer.Write(*resultItem)
	}
	if err := reader.GetError(); err != nil {
		return nil, err
	}
	reader.Reset()
	contentReader = content.NewContentReader(writer.GetFilePath(), writer.GetArrayKey())
	return
}

func SearchFiles(servicesManager artifactory.ArtifactoryServicesManager, spec *spec.SpecFiles) (searchResults []*content.ContentReader, callbackFunc func() error, err error) {
	callbackFunc = func() error {
		var errs error
		for _, reader := range searchResults {
			errs = errors.Join(errs, reader.Close())
		}
		return errs
	}

	var curSearchParams services.SearchParams
	var curReader *content.ContentReader
	for i := 0; i < len(spec.Files); i++ {
		curSearchParams, err = GetSearchParams(spec.Get(i))
		if err != nil {
			return
		}
		curReader, err = servicesManager.SearchFiles(curSearchParams)
		if err != nil {
			return
		}
		searchResults = append(searchResults, curReader)
	}
	return
}
