/*
 *  Copyright IBM Corporation 2021
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package filesystem

import (
	"bytes"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/konveyor/move2kube/common"
	"github.com/sirupsen/logrus"
)

const (
	// SpecialOpeningDelimiter is custom opening delimiter used in golang templates
	SpecialOpeningDelimiter = "<~"
	// SpecialClosingDelimiter is custom closing delimiter used in golang templates
	SpecialClosingDelimiter = "~>"
)

// AddOnConfig bundles the delimiter configuration with template configuration
type AddOnConfig struct {
	OpeningDelimiter string
	ClosingDelimiter string
	Config           interface{}
}

// TemplateCopy copies a directory to another and applies a template config on all files in the directory
func TemplateCopy(source, destination string, config interface{}) error {
	options := options{
		processFileCallBack: templateCopyProcessFileCallBack,
		additionCallBack:    templateCopyAdditionCallBack,
		deletionCallBack:    templateCopyDeletionCallBack,
		mismatchCallBack:    templateCopyDeletionCallBack,
		config:              config,
	}
	return newProcessor(options).process(source, destination)
}

func templateCopyProcessFileCallBack(sourceFilePath, destinationFilePath string, addOnConfigAsIface interface{}) error {
	addOnConfig := AddOnConfig{}
	err := common.GetObjFromInterface(addOnConfigAsIface, &addOnConfig)
	if err != nil {
		logrus.Errorf("Unable to get addOnConfig : %s", err)
		return err
	}
	si, err := os.Stat(sourceFilePath)
	if err != nil {
		logrus.Errorf("Unable to stat file %s : %s", sourceFilePath, err)
		return err
	}
	destinationFilePath, err = common.GetStringFromTemplate(destinationFilePath, addOnConfig.Config)
	if err != nil {
		logrus.Errorf("Unable to fill the template of file path %s : %s", destinationFilePath, err)
		return err
	}
	di, err := os.Stat(destinationFilePath)
	if err == nil {
		if err == nil && !(si.Mode().IsRegular() != di.Mode().IsRegular() || si.Size() != di.Size() || si.ModTime() != di.ModTime()) {
			return nil
		} else if err != nil {
			logrus.Errorf("Unable to compare files to check if files are same %s and %s. Copying normally : %s", sourceFilePath, destinationFilePath, err)
		}
	}
	src, err := os.ReadFile(sourceFilePath)
	if err != nil {
		logrus.Errorf("Unable to open file %s : %s", sourceFilePath, err)
		return err
	}
	destinationWriter, err := os.Create(destinationFilePath)
	if err != nil {
		sdi, err := os.Stat(filepath.Dir(sourceFilePath))
		if err != nil {
			logrus.Errorf("Unable to stat parent dir of %s : %s", sourceFilePath, err)
			return err
		}
		if mderr := os.MkdirAll(filepath.Dir(destinationFilePath), sdi.Mode()); mderr == nil {
			destinationWriter, err = os.Create(destinationFilePath)
		}
		if err != nil {
			logrus.Errorf("Unable to create destination file %s : %s", destinationFilePath, err)
			return err
		}
	}
	defer destinationWriter.Close()
	err = writeTemplateToFile(string(src), addOnConfig.Config,
		destinationFilePath, si.Mode(),
		addOnConfig.OpeningDelimiter, addOnConfig.ClosingDelimiter)
	if err != nil {
		logrus.Errorf("Unable to copy templated file %s to %s : %s", sourceFilePath, destinationFilePath, err)
		return err
	}
	return nil
}

func templateCopyAdditionCallBack(source, destination string, config interface{}) error {
	return nil
}

func templateCopyDeletionCallBack(source, destination string, addOnConfigAsIface interface{}) error {
	addOnConfig := AddOnConfig{}
	err := common.GetObjFromInterface(addOnConfigAsIface, &addOnConfig)
	if err != nil {
		logrus.Errorf("Unable to get addOnConfig : %s", err)
		return err
	}
	si, err := os.Stat(source)
	if err != nil {
		logrus.Errorf("Unable to stat source-path [%s] while detecting template copy: %s", source, err)
		return err
	}
	destination, err = common.GetStringFromTemplate(destination, addOnConfig.Config)
	if err != nil {
		logrus.Errorf("Unable to fill the template of file path %s : %s", destination, err)
		return err
	}
	os.RemoveAll(destination)
	err = os.MkdirAll(destination, si.Mode())
	if err != nil {
		logrus.Errorf("Unable to create directory %s", destination)
		return err
	}
	err = os.Chmod(destination, si.Mode())
	if err != nil {
		logrus.Errorf("Unable to copy permissions in file %s : %s", destination, err)
		return err
	}
	return nil
}

// writeTemplateToFile writes a templated string to a file
func writeTemplateToFile(tpl string, config interface{}, writepath string,
	filemode os.FileMode, openingDelimiter string, closingDelimiter string) error {
	var tplbuffer bytes.Buffer
	if openingDelimiter == "" || closingDelimiter == "" {
		openingDelimiter = "{{"
		closingDelimiter = "}}"
	}
	packageTemplate, err := template.New("").Delims(openingDelimiter, closingDelimiter).Funcs(sprig.TxtFuncMap()).Parse(tpl)
	if err != nil {
		logrus.Errorf("Unable to parse the template : %s", err)
		return err
	}
	err = packageTemplate.Execute(&tplbuffer, config)
	if err != nil {
		logrus.Warnf("Unable to transform template %q to string using the data %v", tpl, config)
		return err
	}
	err = os.WriteFile(writepath, tplbuffer.Bytes(), filemode)
	if err != nil {
		logrus.Warnf("Error writing file at %s : %s", writepath, err)
		return err
	}
	err = os.Chmod(writepath, filemode)
	if err != nil {
		logrus.Warnf("Error writing changing permissions at %s : %s", writepath, err)
		return err
	}
	return nil
}
