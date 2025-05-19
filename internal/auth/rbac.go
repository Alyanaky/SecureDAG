package auth

type Role string

const (
    RoleAdmin  Role = "admin"
    RoleUser   Role = "user"
    RoleViewer Role = "viewer"
)

type Policy struct {
    Resource string
    Actions  []string
}

var RolePolicies = map[Role][]Policy{
    RoleAdmin: {
        {Resource: "*", Actions: []string{"*"}},
    },
    RoleUser: {
        {Resource: "/objects/*", Actions: []string{"PUT", "GET"}},
    },
    RoleViewer: {
        {Resource: "/objects/*", Actions: []string{"GET"}},
    },
}

func HasPermission(role Role, resource string, action string) bool {
    for _, policy := range RolePolicies[role] {
        if (policy.Resource == resource || policy.Resource == "*") && 
           (contains(policy.Actions, action) || contains(policy.Actions, "*")) {
            return true
        }
    }
    return false
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
