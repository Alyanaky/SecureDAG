// Добавить в структуру
type BadgerStore struct {
	db        *badger.DB
	publicKey  *rsa.PublicKey
	privateKey *rsa.PrivateKey
}

// Обновить конструктор
func NewBadgerStore(path string) (*BadgerStore, error) {
	// ... существующий код ...
	
	// Генерируем ключи при инициализации
	privKey, pubKey, err := crypto.GenerateRSAKeys()
	if err != nil {
		return nil, err
	}
	
	return &BadgerStore{
		db:         db,
		publicKey:  pubKey,
		privateKey: privKey,
	}, nil
}

// Обновить методы работы с данными
func (s *BadgerStore) PutBlock(data []byte) (string, error) {
	// Генерируем случайный AES-ключ для каждого блока
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", err
	}

	// Шифруем данные
	encryptedData, err := crypto.EncryptData(data, aesKey)
	if err != nil {
		return "", err
	}

	// Шифруем AES-ключ
	encryptedKey, err := crypto.EncryptAESKey(s.publicKey, aesKey)
	if err != nil {
		return "", err
	}

	// Сохраняем в BadgerDB
	hash := sha256.Sum256(encryptedData)
	key := fmt.Sprintf("%x", hash)
	
	err = s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set([]byte(key), encryptedData); err != nil {
			return err
		}
		return txn.Set([]byte("key_"+key), encryptedKey)
	})
	
	return key, err
}

func (s *BadgerStore) GetBlock(hash string) ([]byte, error) {
	var encryptedData, encryptedKey []byte
	
	// Получаем зашифрованные данные
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(hash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			encryptedData = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Получаем зашифрованный ключ
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("key_"+hash))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			encryptedKey = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Расшифровываем AES-ключ
	aesKey, err := crypto.DecryptAESKey(s.privateKey, encryptedKey)
	if err != nil {
		return nil, err
	}

	// Расшифровываем данные
	return crypto.DecryptData(encryptedData, aesKey)
}
