package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"
)

type testRatingDB struct {
	RatingDB
	create func(*Rating) error
}

func (t *testRatingDB) Create(mr *Rating) error {
	if t.create != nil {
		return t.create(mr)
	}

	return nil
}

func dropRatingsTable(db *gorm.DB) {
	db.DropTableIfExists(&Rating{})
}

func TestRatingService_Create(t *testing.T) {
	rtdb := &testRatingDB{}
	rs := NewRatingService(nil)
	rs.(*ratingService).RatingService.(*ratingValidator).RatingDB = rtdb

	var cases = []struct {
		name      string
		rating    *Rating
		outrating *Rating
		outerr    error
		setup     func(t *testing.T)
	}{
		{
			"targetRequired",
			&Rating{Score: 10, UserID: 1},
			nil,
			ValidationError{"target": ErrRequired},
			nil,
		},
		{
			"scoreRequired",
			&Rating{Target: 999, UserID: 1},
			nil,
			ValidationError{"score": ErrRequired},
			nil,
		},
		{
			"userIdRequired",
			&Rating{Score: 10, Target: 999},
			nil,
			ValidationError{"userId": ErrRequired},
			nil,
		},
		{
			"commentTooLong",
			&Rating{
				Comment: strings.Repeat("a", 513),
				Score:   10,
				Target:  999,
				UserID:  1,
			},
			nil,
			ValidationError{"comment": ErrTooLong},
			nil,
		},
		{
			"extraTooLong",
			&Rating{
				Extra: json.RawMessage(`{
					"too_long":"` + strings.Repeat("a", 512) + `"
				}`),
				Score:  10,
				Target: 999,
				UserID: 1,
			},
			nil,
			ValidationError{"extra": ErrTooLong},
			nil,
		},
		{
			"targetInvalid",
			&Rating{Score: 10, Target: -999, UserID: 1},
			nil,
			ValidationError{"target": ErrInvalid},
			nil,
		},
		{
			"userIdInvalid",
			&Rating{Score: 10, Target: 999, UserID: -999},
			nil,
			ValidationError{"userId": ErrInvalid},
			nil,
		},
		{
			"ok",
			&Rating{Score: 10, Target: 999, UserID: 1},
			&Rating{
				ID:        99,
				Active:    true,
				Anonymous: true,
				Extra:     json.RawMessage(`{}`),
				Score:     10,
				Target:    999,
				UserID:    1,
			},
			nil,
			func(t *testing.T) {
				rtdb.create = func(rt *Rating) error {
					var crt Rating
					crt.Score = 10
					crt.Target = 999
					crt.UserID = 1

					assert.NotZero(t, rt.Date)
					rt.Date = 0 // Remove date to avoid mismatch

					assert.Equal(t, &crt, rt)

					*rt = NewRating()
					rt.Score = 10
					rt.Target = 999
					rt.UserID = 1

					rt.ID = 99
					return nil
				}
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			if cs.setup != nil {
				cs.setup(t)
			}

			err := rs.Create(cs.rating)

			if cs.outerr != nil {
				assert.Error(t, err)
				assert.True(t, xerrors.Is(err, cs.outerr),
					"errors must match, expected %v, got %v", cs.outerr, err)

			} else {
				assert.NoError(t, err)
				assert.Equal(t, cs.outrating, cs.rating)
			}
		})
	}
}

func TestRatingGORM_Create(t *testing.T) {
	var cases = []struct {
		name   string
		rating *Rating

		outerr error

		setup func(t *testing.T, db *gorm.DB)
	}{
		{
			"ok",
			&Rating{
				ID:        99,
				Active:    true,
				Anonymous: true,
				Comment:   "",
				Date:      1257894000000,
				Extra:     json.RawMessage(`{}`),
				Score:     10,
				Target:    6345,
				UserID:    1,
			},
			nil,
			nil,
		},
		{
			"repeatedID",
			&Rating{ID: 99, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			ValidationError{"id": ErrIDTaken},
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(
					&Rating{
						ID:        99,
						Active:    true,
						Anonymous: true,
						Comment:   "",
						Date:      1257894000000,
						Extra:     json.RawMessage(`{}`),
						Score:     10,
						Target:    6345,
						UserID:    1,
					},
				).Error)
			},
		},
		{
			"fkViolationUser",
			&Rating{
				ID:        99,
				Active:    true,
				Anonymous: true,
				Comment:   "",
				Date:      1257894000000,
				Extra:     json.RawMessage(`{}`),
				Score:     10,
				Target:    6345,
				UserID:    999,
			},
			ValidationError{"userId": ErrRefNotFound},
			nil,
		},
		{
			"multipleRatingsSameProductSameUser",
			&Rating{
				ID:        99,
				Active:    true,
				Anonymous: true,
				Comment:   "",
				Date:      1257894000000,
				Extra:     json.RawMessage(`{}`),
				Score:     10,
				Target:    6345,
				UserID:    1,
			},
			ValidationError{"target": ErrDuplicate},
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(
					&Rating{
						ID:        77,
						Active:    true,
						Anonymous: true,
						Comment:   "",
						Date:      1257894000000,
						Extra:     json.RawMessage(`{}`),
						Score:     10,
						Target:    6345,
						UserID:    1,
					},
				).Error)
			},
		},
		{
			"multipleRatingSameProductDifferentUser",
			&Rating{
				ID:        99,
				Active:    true,
				Anonymous: true,
				Comment:   "",
				Date:      1257894000000,
				Extra:     json.RawMessage(`{}`),
				Score:     10,
				Target:    6345,
				UserID:    1,
			},
			nil,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(
					&User{
						Active:    true,
						RoleID:    2,
						Email:     "test@test.com",
						FirstName: "Test",
						LastName:  "User",
						Password:  "TestPasswordHAsh",
						Settings:  "Settings string here",
					},
				).Error)

				require.NoError(t, db.Create(
					&Rating{
						ID:        77,
						Active:    true,
						Anonymous: true,
						Comment:   "",
						Date:      1257894000000,
						Extra:     json.RawMessage(`{}`),
						Score:     10,
						Target:    6345,
						UserID:    2,
					},
				).Error)
			},
		},
		{
			"internalError",
			&Rating{ID: 99, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			privateError("some internal error"),
			func(t *testing.T, db *gorm.DB) {
				dropRatingsTable(db)
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			db := setupGorm(t)

			if cs.setup != nil {
				cs.setup(t, db)
			}

			err := (&ratingGorm{db}).Create(cs.rating)

			if cs.outerr != nil {
				assert.Error(t, err)
				if _, ok := cs.outerr.(PublicError); ok {
					assert.True(t, xerrors.Is(err, cs.outerr))
				}

			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, int64(0), cs.rating.ID, "must set the ID")
			}
		})
	}
}

func TestRatingGORM_Update(t *testing.T) {
	var cases = []struct {
		name   string
		rating *Rating
		outerr error
		setup  func(t *testing.T, db *gorm.DB)
	}{
		{
			"ok",
			&Rating{ID: 999, Active: false, Anonymous: false, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"idNotExists",
			&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			ErrNotFound,
			nil,
		},
		{
			"noChanges",
			&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"changesDefaultGoValues",
			&Rating{ID: 999, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: false, Anonymous: false, Comment: "", Date: 0, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"internalError",
			&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			privateError("any internal private error"),
			func(t *testing.T, db *gorm.DB) {
				dropRatingsTable(db)
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			db := setupGorm(t)

			if cs.setup != nil {
				cs.setup(t, db)
			}

			err := (&ratingGorm{db}).Update(cs.rating)

			if cs.outerr != nil {
				assert.Error(t, err)
				if _, ok := cs.outerr.(PublicError); ok {
					assert.True(t, xerrors.Is(err, cs.outerr))
					fmt.Println(err)
				}

			} else {
				assert.NoError(t, err)

				var urating Rating
				require.NoError(t, db.First(&urating, cs.rating.ID).Error)
				assert.Equal(t, cs.rating, &urating)
			}
		})
	}
}

func TestRatingGORM_Delete(t *testing.T) {
	var cases = []struct {
		name   string
		rating *Rating
		outerr error
		setup  func(t *testing.T, db *gorm.DB)
	}{
		{
			"ok",
			&Rating{ID: 999, Active: false, Anonymous: false, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"notFound",
			&Rating{ID: 999},
			ErrNotFound,
			nil,
		},
		{
			"internalError",
			&Rating{ID: 999},
			privateError("any internal private error"),
			func(t *testing.T, db *gorm.DB) {
				dropRatingsTable(db)
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			db := setupGorm(t)

			if cs.setup != nil {
				cs.setup(t, db)
			}

			err := (&ratingGorm{db}).Delete(cs.rating.ID)

			if cs.outerr != nil {
				assert.Error(t, err)
				if _, ok := cs.outerr.(PublicError); ok {
					assert.True(t, xerrors.Is(err, cs.outerr))
					fmt.Println(err)
				}

			} else {
				assert.NoError(t, err)
			}

		})
	}
}

func TestRatingGORM_ByID(t *testing.T) {
	var cases = []struct {
		name    string
		queryID int64
		rating  *Rating
		outerr  error
		equal   bool // Flag to assert Equal/Non-equal elements
		setup   func(t *testing.T, db *gorm.DB)
	}{
		{
			"ok",
			999,
			&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			true,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"notEqual",
			999,
			&Rating{ID: 999, Active: false, Anonymous: false, Comment: "", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			nil,
			false,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"notFound",
			999,
			nil,
			ErrNotFound,
			true,
			nil,
		},
		{
			"internalError",
			999,
			nil,
			privateError("any internal private error"),
			true,
			func(t *testing.T, db *gorm.DB) {
				dropRatingsTable(db)
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			db := setupGorm(t)

			if cs.setup != nil {
				cs.setup(t, db)
			}

			r, err := (&ratingGorm{db}).ByID(cs.queryID)

			if cs.outerr != nil {
				assert.Error(t, err)
				if _, ok := cs.outerr.(PublicError); ok {
					assert.True(t, xerrors.Is(err, cs.outerr))
					fmt.Println(err)
				}

			} else {
				if !cs.equal {
					assert.NotEqual(t, cs.rating, &r)
				} else {
					assert.NoError(t, err)

					assert.Equal(t, cs.rating, &r)
				}
			}

		})
	}
}

func TestRatingGORM_ByTarget(t *testing.T) {
	var cases = []struct {
		name    string
		queryID int64
		rating  *[]Rating
		outerr  error
		// equal   bool // Flag to assert Equal/Non-equal elements
		setup func(t *testing.T, db *gorm.DB)
	}{
		{
			"ok",
			6345,
			&[]Rating{
				Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			},
			nil,
			// true,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
			},
		},
		{
			"getOne",
			6345,
			&[]Rating{
				Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
			},
			nil,
			// false,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
				require.NoError(t, db.Create(&Rating{ID: 888, Active: true, Anonymous: true, Comment: "Awesome too", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 8974, UserID: 1}).Error)
			},
		},
		{
			"getMultiple",
			6345,
			&[]Rating{
				Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1},
				Rating{ID: 888, Active: true, Anonymous: true, Comment: "Awesome too", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 99},
			},
			nil,
			// false,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&User{ID: 99, RoleID: 2, Email: "second@test.com", FirstName: "Second", Password: "TestPasswordHAshOther"}).Error)
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
				require.NoError(t, db.Create(&Rating{ID: 888, Active: true, Anonymous: true, Comment: "Awesome too", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 99}).Error)
			},
		},
		{
			"notFound1",
			9999,
			&[]Rating{},
			nil,
			// true,
			func(t *testing.T, db *gorm.DB) {
				require.NoError(t, db.Create(&Rating{ID: 999, Active: true, Anonymous: true, Comment: "Awesome", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 6345, UserID: 1}).Error)
				require.NoError(t, db.Create(&Rating{ID: 888, Active: true, Anonymous: true, Comment: "Awesome too", Date: 1257894000000, Extra: json.RawMessage(`{}`), Score: 10, Target: 8974, UserID: 1}).Error)
			},
		},
		{
			"notFound2",
			999,
			&[]Rating{},
			nil,
			// true,
			nil,
		},
		{
			"internalError",
			999,
			nil,
			privateError("any internal private error"),
			// true,
			func(t *testing.T, db *gorm.DB) {
				dropRatingsTable(db)
			},
		},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			db := setupGorm(t)

			if cs.setup != nil {
				cs.setup(t, db)
			}

			r, err := (&ratingGorm{db}).ByTarget(cs.queryID)

			if cs.outerr != nil {
				assert.Error(t, err)
				if _, ok := cs.outerr.(PublicError); ok {
					assert.True(t, xerrors.Is(err, cs.outerr))
					fmt.Println(err)
				}

			} else {
				// if !cs.equal {
				// 	assert.NotEqual(t, cs.rating, &r)
				// } else {
				assert.NoError(t, err)
				assert.Equal(t, cs.rating, &r)
				// assert.Equal(t, len(*cs.rating), len(r), "Same length is expected when asserting a valid value.")
				// }
			}

		})
	}
}