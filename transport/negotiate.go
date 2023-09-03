package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	json "github.com/json-iterator/go"
)

/* negotiate protocol
magic:   1 byte
dataLen: 2 bytes
data:    > 0 bytes
*/

const negotiateMagic byte = 0x5F

type clientHello struct {
	Host   string `json:"host"`
	Secret string `json:"secret"`
}

type serverHello struct {
	Host string `json:"host"`
	Id   string `json:"id"`
}

func ClientNegotiate(c io.ReadWriter, host string, credential string) (peer string, id string, err error) {
	r := &clientHello{
		Host:   host,
		Secret: credential,
	}

	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	err = writeData(c, data)
	if err != nil {
		return
	}

	buffer, err := readData(c)
	if err != nil {
		return
	}

	var s = &serverHello{}
	err = json.Unmarshal(buffer, s)
	if err != nil {
		return
	}
	return s.Host, s.Id, nil
}

func ServerNegotiate(c io.ReadWriter, host string, id string, validator func(credential string) error) (peer string, err error) {
	data, err := readData(c)
	if err != nil {
		return
	}

	var r = &clientHello{}
	err = json.Unmarshal(data, r)
	if err != nil {
		return
	}

	if validator != nil {
		if err = validator(r.Secret); err != nil {
			err = fmt.Errorf("server negotiate credential validation failure, %v", err)
			return
		}
	}

	var s = &serverHello{
		Host: host,
		Id:   id,
	}

	data, err = json.Marshal(s)
	if err != nil {
		return
	}

	err = writeData(c, data)
	if err != nil {
		return
	}

	return r.Host, nil
}

func writeData(c io.Writer, data []byte) error {
	buffer := make([]byte, 3+len(data))
	buffer[0] = negotiateMagic
	binary.BigEndian.PutUint16(buffer[1:], uint16(len(data)))
	copy(buffer[3:], data)
	_, err := c.Write(buffer)
	return err
}

func readData(c io.Reader) ([]byte, error) {
	buf := make([]byte, 3)
	_, err := io.ReadFull(c, buf)
	if err != nil {
		return nil, err
	}

	if buf[0] != negotiateMagic {
		return nil, errors.New("not negotiation package")
	}
	dataLen := int(binary.BigEndian.Uint16(buf[1:]))
	buf = make([]byte, dataLen)

	_, err = io.ReadFull(c, buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
