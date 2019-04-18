package util

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path"
)

func ZipDirectory(outfile, dir string) error {
	zf, err := os.Create(outfile)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(zf)

	err = addFiles(zw, outfile, dir, "")
	if err != nil {
		return err
	}

	err = zw.Close()
	if err != nil {
		return err
	}

	err = zf.Close()
	if err != nil {
		return err
	}

	return nil
}

func addFiles(w *zip.Writer, outfile, basePath, baseInZip string) error {
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if path.Join(basePath, file.Name()) == outfile {
			continue
		}

		if file.IsDir() {
			newBase := path.Join(basePath, file.Name())

			err = addFiles(w, outfile, newBase, file.Name())
			if err != nil {
				return err
			}

			continue
		}

		f, err := os.Open(path.Join(basePath, file.Name()))
		if err != nil {
			return err
		}

		zw, err := w.Create(path.Join(baseInZip, file.Name()))
		if err != nil {
			return err
		}

		_, err = io.Copy(zw, f)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
