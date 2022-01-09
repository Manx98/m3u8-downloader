package util

import (
	"crypto/aes"
	"crypto/cipher"
)

//  ============================= 加解密相关 ===========================

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unPadding := int(origData[length-1])
	return origData[:(length - unPadding)]
}

func AesDecrypt(encrypted, key []byte, ivs ...[]byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	var iv []byte
	if len(ivs) == 0 {
		iv = key
	} else {
		iv = ivs[0]
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(encrypted))
	blockMode.CryptBlocks(origData, encrypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}
