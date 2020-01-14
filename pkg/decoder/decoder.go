package decoder

import (
	"bufio"
	"bytes"
	"io"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type YamlDecoder struct {
	reader  *yaml.YAMLReader
	decoder runtime.Decoder
	close   func() error
}

// Modified from https://github.com/kubernetes-sigs/cluster-api. Improved to accommodate custom schemes.
func (d *YamlDecoder) Decode(defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, []byte, error) {
	for {
		doc, err := d.reader.Read()
		if err != nil {
			return nil, nil, err
		}

		//  Skip over empty documents, i.e. a leading `---`
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		obj, _, err := d.decoder.Decode(doc, defaults, into)
		return obj, doc, err
	}
}

func (d *YamlDecoder) Close() error {
	return d.close()
}

func NewYAMLDecoder(r io.ReadCloser, scheme *runtime.Scheme) *YamlDecoder {
	return &YamlDecoder{
		reader:  yaml.NewYAMLReader(bufio.NewReader(r)),
		decoder: serializer.NewCodecFactory(scheme).UniversalDeserializer(),
		close:   r.Close,
	}
}
