package database

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/l3uddz/wantarr/logger"
	"github.com/l3uddz/wantarr/pvr"
	stringutils "github.com/l3uddz/wantarr/utils/strings"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	log = logger.GetLogger("db")
)

type Database struct {
	// private
	filePath string
	log      *logrus.Entry
	vault    map[int]pvr.MediaItem
	changed  bool
	loaded   bool
}

func New(name string, suffix string, databaseFolder string) (*Database, error) {
	db := &Database{
		filePath: filepath.Join(databaseFolder, fmt.Sprintf("%s_%s.json", name, suffix)),
		log:      logger.GetLogger(fmt.Sprintf("db.%s", name)),
		vault:    make(map[int]pvr.MediaItem, 0),
		changed:  false,
		loaded:   false,
	}

	// show log
	log.Infof("Using %s = %q", stringutils.StringLeftJust("DATABASE", " ", 10), db.filePath)

	// does database file already exist?
	if _, err := os.Stat(db.filePath); os.IsNotExist(err) {
		return db, nil
	}

	// open database file
	dbFile, err := os.Open(db.filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed loading database file: %q", db.filePath)
	}
	defer dbFile.Close()

	// read database data
	dbData, err := ioutil.ReadAll(dbFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading bytes from database file: %q", db.filePath)
	}

	// unmarshal cache data
	if err := json.Unmarshal(dbData, &db.vault); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal bytes from database file: %q", db.filePath)
	}

	db.loaded = true

	return db, nil
}
