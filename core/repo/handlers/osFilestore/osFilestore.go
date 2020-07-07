package osFilestore

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aloknerurkar/gopcp/core"
	uuid "github.com/google/uuid"
)

type osFileStore struct {
	root string
}

func NewOSFileStore(conf map[string]interface{}) (core.Store, error) {

	store := new(osFileStore)

	rootPath, ok := conf["root"].(string)
	if !ok {
		return nil, errors.New("Root path missing in config")
	}

	dbName, ok := conf["dbName"].(string)
	if !ok {
		return nil, errors.New("DB name missing in config")
	}

	store.root = rootPath + "/" + dbName
	err := os.Mkdir(store.root, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	return store, nil
}

func (s *osFileStore) getFullPath(i core.Item) string {
	return s.root + "/" + i.GetTable() + "/" + i.GetId()
}

func (s *osFileStore) getParentPath(i core.Item) string {
	return s.root + "/" + i.GetTable()
}

func (s *osFileStore) Create(i core.Item) error {

	if v, ok := i.(core.IdSetter); ok {
		v.SetId(uuid.New().String())
	}

	if fi, ok := i.(core.FileItemSetter); ok {
		if _, err := os.Stat(s.getParentPath(i)); os.IsNotExist(err) {
			os.Mkdir(s.getParentPath(i), os.ModePerm)
		}
		fp, err := os.Create(s.getFullPath(i))
		if err != nil {
			return fmt.Errorf("failed creating file %v", err)
		}
		fi.SetFp(fp)
		return nil
	}

	return errors.New("Invalid item type")
}

func (s *osFileStore) Update(i core.Item) error {
	if fi, ok := i.(core.FileItemSetter); ok {
		fp, err := os.OpenFile(s.getFullPath(i), os.O_WRONLY, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed opening file for edit %v", err)
		}
		fi.SetFp(fp)
		return nil
	}

	return errors.New("Invalid item type")
}

func (s *osFileStore) Delete(i core.Item) error {
	return os.Remove(s.getFullPath(i))
}

func (s *osFileStore) Read(i core.Item) error {
	if fi, ok := i.(core.FileItemSetter); ok {
		fp, err := os.Open(s.getFullPath(i))
		if err != nil {
			return fmt.Errorf("failed opening file for read %v", err)
		}
		fi.SetFp(fp)
		return nil
	}

	return errors.New("Invalid item type")
}

func (s *osFileStore) List(l core.Items, o core.ListOpt) (int, error) {
	if len(l) == 0 {
		return 0, nil
	}
	files, err := ioutil.ReadDir(s.getParentPath(l[0]))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("error opening root dir %s Err:%v", s.getParentPath(l[0]), err)
	}

	if o.Page*o.Limit >= int64(len(files)) {
		// No more files. Not returning error as of now.
		return 0, nil
	}

	skip := o.Page * o.Limit
	for i := skip; i < skip+o.Limit+1 && i < int64(len(files)); i++ {
		var fp *os.File
		if fi, ok := l[i].(core.FileItemSetter); ok {
			fp, err = os.Open(files[i].Name())
			if err == nil {
				fi.SetFp(fp)
			}
		} else {
			err = errors.New("File not provided.")
		}
		if err != nil {
			err = fmt.Errorf("Failed iterating over files Err:%v", err)
			break
		}
	}
	return len(l), err
}

func (s *osFileStore) ListAll(allocFn func(i int) core.Items) (core.Items, error) {
	it := allocFn(1)
	files, err := ioutil.ReadDir(s.getParentPath(it[0]))
	if err != nil {
		if os.IsNotExist(err) {
			// Returning empty
			return allocFn(0), nil
		}
		return nil, fmt.Errorf("error opening root dir %s Err:%v", s.root, err)
	}

	l := allocFn(len(files))

	for i, v := range files {
		var fp *os.File
		if fi, ok := l[i].(core.FileItemSetter); ok {
			fp, err = os.Open(v.Name())
			if err != nil {
				return nil, err
			}
			fi.SetFp(fp)
		} else {
			err = errors.New("Object type not allocated correctly.")
			return nil, err
		}
	}
	return l, err
}
