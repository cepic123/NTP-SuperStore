package main

import (
	"database/sql"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type StorageInterface interface {
	CreateRating(*Rating) error
	UpdateRating(*Rating) error
	GetRatingBySubjectAndUser(int, int, string) (*Rating, error)
	GetSubjectRating(int, string) (int, error)
}

func (s *Storage) GetRatingBySubjectAndUser(userId, subjectId int, ratingType string) (*Rating, error) {
	var rating Rating
	result := s.db.Where(&Rating{UserID: userId, SubjectID: subjectId, RatingType: ratingType}).Find(&rating)

	if result.Error != nil {
		return nil, result.Error
	}
	return &rating, nil
}

func (s *Storage) GetSubjectRating(subjectId int, ratingType string) (int, error) {
	var ratings []Rating
	result := s.db.Where(&Rating{SubjectID: subjectId, RatingType: ratingType}).Find(&ratings)

	if result.Error != nil {
		return 0, result.Error
	}

	sum := 0
	for i := 0; i < len(ratings); i++ {
		sum += ratings[i].Rating
	}

	return sum / len(ratings), nil
}

func (s *Storage) CreateRating(rating *Rating) error {
	if result := s.db.Create(rating); result.Error != nil {
		return result.Error
	}
	return nil
}

func (s *Storage) UpdateRating(rating *Rating) error {
	if result := s.db.Save(rating); result.Error != nil {
		return result.Error
	}
	return nil
}

type Storage struct {
	db *gorm.DB
}

func NewStorage() (*Storage, error) {
	connStr := "user=postgres dbname=ratingDB password=root sslmode=disable"
	PgDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := PgDB.Ping(); err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: PgDB,
	}), &gorm.Config{})

	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Rating{})

	return &Storage{
		db: db,
	}, nil
}
