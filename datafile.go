package elasticthought

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/couchbaselabs/cbfs/client"
	"github.com/couchbaselabs/logg"
	"github.com/tleyden/go-couch"
)

// A Datafile is a raw "bundle" of data, typically a zip or .tar.gz file.
// It cannot be used by a solver directly, instead it used to create
// dataset objects which can be used by the solver.
// A single datafile can be used to create any number of dataset objects.
type Datafile struct {
	ElasticThoughtDoc
	UserID string `json:"user-id"`
	Url    string `json:"url" binding:"required"`
}

// Create a new datafile
func NewDatafile() *Datafile {
	return &Datafile{
		ElasticThoughtDoc: ElasticThoughtDoc{Type: DOC_TYPE_DATAFILE},
	}
}

// Find Datafile by Id from the db
func FindDatafile(db couch.Database, datafileId string) (*Datafile, error) {

	datafile := &Datafile{}
	if err := db.Retrieve(datafileId, datafile); err != nil {
		return nil, err
	}
	return datafile, nil

}

// Save a new version of Datafile to the db
func (d Datafile) Save(db couch.Database) (*Datafile, error) {

	idToRetrieve := ""

	switch d.HasValidId() {
	case true:
		logg.LogTo("MODEL", "calling db.Edit()")
		_, err := db.Edit(d)
		if err != nil {
			return nil, err
		}
		idToRetrieve = d.Id
	default:
		logg.LogTo("MODEL", "calling db.Insert()")
		id, _, err := db.Insert(d)
		if err != nil {
			return nil, err
		}
		idToRetrieve = id
	}

	// load latest version from db to get the _id and _rev fields
	datafile := &Datafile{}
	err := db.Retrieve(idToRetrieve, datafile)
	if err != nil {
		return nil, err
	}

	return datafile, nil

}

// Does this datafile have a valid Id?
func (d Datafile) HasValidId() bool {
	return len(d.Id) > 0
}

// Copy the contents of Datafile.Url to CBFS and update the Datafile.Url to point to that
func (d Datafile) CopyToCBFS(db couch.Database, cbfs *cbfsclient.Client) {

	if !d.HasValidId() {
		errMsg := fmt.Errorf("Datafile: %+v must have an id", d)
		logg.LogError(errMsg)
		return
	}

	// figure out dest path to save to on cbfs
	u, err := url.Parse(d.Url)
	if err != nil {
		errMsg := fmt.Errorf("Error parsing: %v. Err %v", d.Url, err)
		logg.LogError(errMsg)
		return
	}
	urlPath := u.Path
	_, filename := path.Split(urlPath)
	destPath := fmt.Sprintf("%v/%v", d.Id, filename)

	// open input stream to url
	resp, err := http.Get(d.Url)
	defer resp.Body.Close()
	if err != nil {
		errMsg := fmt.Errorf("Error opening: %v. Err %v", d.Url, err)
		logg.LogError(errMsg)
		return
	}

	// write to cbfs
	options := cbfsclient.PutOptions{
		ContentType: resp.Header.Get("Content-Type"),
	}
	if err := cbfs.Put("", destPath, resp.Body, options); err != nil {
		errMsg := fmt.Errorf("Error writing %v to cbfs: %v", destPath, err)
		logg.LogError(errMsg)
		return
	}

	// update datafile with new url
	d.Url = fmt.Sprintf("%v%v", CBFS_URI_PREFIX, destPath)
	_, err = d.Save(db)
	if err != nil {
		errMsg := fmt.Errorf("Error saving: %+v. Err %v", d, err)
		logg.LogError(errMsg)
		return
	}

}
