package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost is calibrated for desktop hardware — high enough that an
// offline attacker on a leaked DB pays seconds per guess, low enough
// that login on a 2018 laptop stays under ~250ms. Tune via env if
// deploying server-side.
const bcryptCost = 12

func hashPassword(plain string) (string, error) {
	if len(plain) < 12 {
		return "", ErrWeakPassword
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func verifyPassword(hash, plain string) bool {
	if hash == "" || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
