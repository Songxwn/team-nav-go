package auth

import "golang.org/x/crypto/bcrypt"

func bcryptCompare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
