package entity

import (
	"time"
)

// Tenants is an array of Tenant
type Tenants []*Tenant

// Tenant defines the tenant
type Tenant struct {
	Id         string
	Uid        string
	Zone       string
	Partition  string
	Priority   int32
	Name       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	LastActive time.Time

	ResourceQuota ResourceQuota
}

func (t *Tenant) Fields() []interface{} {
	return []interface{}{
		&t.Id, &t.Uid, &t.Zone, &t.Priority, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.LastActive,
	}
}

func (t *Tenant) FieldsWithQuota() []interface{} {
	dst := []interface{}{
		&t.Id, &t.Uid, &t.Zone, &t.Priority, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.LastActive,
	}
	dst = append(dst, t.ResourceQuota.Fields())
	return dst
}
