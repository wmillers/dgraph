//go:build !oss
// +build !oss

/*
 * Copyright 2023 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package audit

import (
	"crypto/aes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dgraph-io/dgraph/testutil/testaudit"
)

// we will truncate copy of encrypted audit log file
func copy(t *testing.T, src string, dst string) {
	// could also us io.CopyN but this is a small file
	data, err := os.ReadFile(src)
	require.NoError(t, err)
	err = os.WriteFile(dst, data, 0666)
	require.NoError(t, err)
}

// check whether we can properly decrypt an encryped audit log that is truncated at the tail
func TestDecrypt(t *testing.T) {
	key, err := os.ReadFile("../enc/test-fixtures/enc-key")
	require.NoError(t, err)

	// encrypted audit logs generated by TestGenerateAuditLogForTestDecrypt in testutil/audit.go
	// we check this in because we want this test to be a unit test
	filePath := "testfiles/zero_audit_0_1.log.enc"
	copyPath := "testfiles/zero_audit_0_1.log.enc.copy"
	decryptedPath := "testfiles/zero_audit_0_1.log"

	// during test we will truncate copy of file
	copy(t, filePath, copyPath)
	defer os.RemoveAll(copyPath)

	file, err := os.OpenFile(copyPath, os.O_RDWR, 0666)
	require.NoError(t, err)
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatal("error closing file")
		}
	}()

	stat, err := os.Stat(copyPath)
	require.NoError(t, err)
	sz := stat.Size() // get size of audit log

	outfile, err := os.OpenFile(decryptedPath,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	require.NoError(t, err)
	defer func() {
		if err := outfile.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	defer os.RemoveAll("testfiles/zero_audit_0_1.log")
	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	zeroCmd := []string{"/removeNode", "/assign", "/moveTablet"}

	// size of audit log is 825 bytes
	// keep truncating encrypted audit log file 5 bytes at a time
	for i := 0; i <= int(sz); i = i + 5 {
		switch {
		case i == 0:
			require.NoError(t, decrypt(file, outfile, block, sz))
			testaudit.VerifyLogs(t, decryptedPath, zeroCmd)
			// clear output file
			require.NoError(t, outfile.Truncate(0))
			_, err := outfile.Seek(0, 0)
			require.NoError(t, err)
		case 5 <= i && i <= 275:
			require.NoError(t, file.Truncate(sz-int64(i)))
			require.NoError(t, decrypt(file, outfile, block, sz))
			testaudit.VerifyLogs(t, decryptedPath, zeroCmd[0:1])
			require.NoError(t, outfile.Truncate(0))
			_, err := outfile.Seek(0, 0)
			require.NoError(t, err)
		case 280 <= i && i <= 535:
			require.NoError(t, file.Truncate(sz-int64(i)))
			require.NoError(t, decrypt(file, outfile, block, sz))
			testaudit.VerifyLogs(t, decryptedPath, zeroCmd[0:0])
			require.NoError(t, outfile.Truncate(0))
			_, err := outfile.Seek(0, 0)
			require.NoError(t, err)
		case 540 <= i && i <= 790:
			// at this point the output file will be empty
			require.NoError(t, file.Truncate(sz-int64(i)))
			// verify that decrypt does not panic
			require.NoError(t, decrypt(file, outfile, block, sz))
			require.NoError(t, outfile.Truncate(0))
			_, err := outfile.Seek(0, 0)
			require.NoError(t, err)
		}
	}
}