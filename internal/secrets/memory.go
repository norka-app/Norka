package secrets

import "sync"

// MemoryKeyring is an in-process keychain for tests.
type MemoryKeyring struct {
	mu            sync.Mutex
	items         map[string]string
	failSet       map[string]error
	failSecret    string
	failSecretErr error
}

func NewMemoryKeyring() *MemoryKeyring {
	return &MemoryKeyring{items: map[string]string{}}
}

// FailSet makes Set return err for the given account.
func (m *MemoryKeyring) FailSet(user string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failSet == nil {
		m.failSet = map[string]error{}
	}
	m.failSet[user] = err
}

func (m *MemoryKeyring) key(service, user string) string {
	return service + "\x00" + user
}

// FailIfSecret makes Set return err when the secret equals secret.
func (m *MemoryKeyring) FailIfSecret(secret string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failSecret = secret
	m.failSecretErr = err
}

func (m *MemoryKeyring) Set(service, user, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.failSet[user]; err != nil {
		return err
	}
	if m.failSecretErr != nil && secret == m.failSecret {
		return m.failSecretErr
	}
	if m.items == nil {
		m.items = map[string]string{}
	}
	m.items[m.key(service, user)] = secret
	return nil
}

func (m *MemoryKeyring) Get(service, user string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	secret, ok := m.items[m.key(service, user)]
	if !ok {
		return "", ErrNotFound
	}
	return secret, nil
}

func (m *MemoryKeyring) Delete(service, user string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(service, user)
	if _, ok := m.items[k]; !ok {
		return ErrNotFound
	}
	delete(m.items, k)
	return nil
}
