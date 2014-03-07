package utils

import (
	"appengine"
	"appengine/datastore"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

func GenerateUniqueSlug(ctx appengine.Context, kind string, s string) (slug string) {
	slug = GenerateSlug(s)
	others, err := datastore.NewQuery(kind).
		Filter("Slug = ", slug).
		Count(ctx)
	if err != nil {
		ctx.Errorf("[utils/GenerateUniqueSlug] %v", err.Error())
		return ""
	}
	if others == 0 {
		return slug
	}
	counter := 2
	baseSlug := slug
	for others > 0 {
		slug = fmt.Sprintf("%v-%d", baseSlug, counter)
		others, err = datastore.NewQuery(kind).
			Filter("Slug = ", slug).
			Count(ctx)
		if err != nil {
			ctx.Errorf("[utils/GenerateUniqueSlug] %v", err.Error())
			return ""
		}
		counter = counter + 1
	}
	return slug
}

func GenerateSlug(s string) (slug string) {
	return strings.Map(func(r rune) rune {
		switch {
		case r == ' ', r == '-':
			return '-'
		case r == '_', unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		default:
			return -1
		}
		return -1
	}, strings.ToLower(strings.TrimSpace(s)))
}

// type ApiResponse is a generic API response struct
type ApiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

func Save(c appengine.Context, obj interface{}) (key *datastore.Key, err error) {
	kind, val := reflect.TypeOf(obj), reflect.ValueOf(obj)
	str := val
	if val.Kind().String() == "ptr" {
		kind, str = kind.Elem(), val.Elem()
	}
	if str.Kind().String() != "struct" {
		return nil, errors.New("Must pass a valid object to struct")
	}
	dsKind := kind.String()
	if li := strings.LastIndex(dsKind, "."); li >= 0 {
		//Format kind to be in a standard format used for datastore
		dsKind = dsKind[li+1:]
	}
	if bsMethod := val.MethodByName("BeforeSave"); bsMethod.IsValid() {
		bsMethod.Call([]reflect.Value{reflect.ValueOf(c)})
	}
	//check for key field first
	keyField := str.FieldByName("Key")
	if keyField.IsValid() {
		keyInterface := keyField.Interface()
		key, _ = keyInterface.(*datastore.Key)
	}
	idField := str.FieldByName("ID")
	if key == nil {
		if idField.IsValid() && idField.Int() != 0 {
			key = datastore.NewKey(c, dsKind, "", idField.Int(), nil)
		} else {
			key = datastore.NewIncompleteKey(c, dsKind, nil)
		}
	}
	//Store in memcache
	key, err = datastore.Put(c, key, obj)
	if err != nil {
		c.Errorf("[utils/Save]: %v", err.Error())
	} else {
		if keyField.IsValid() {
			keyField.Set(reflect.ValueOf(key))
		}
		if idField.IsValid() {
			idField.SetInt(key.IntID())
		}
		if asMethod := val.MethodByName("AfterSave"); asMethod.IsValid() {
			asMethod.Call([]reflect.Value{reflect.ValueOf(c), reflect.ValueOf(key)})
		}
	}
	return
}
