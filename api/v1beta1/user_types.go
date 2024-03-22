package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UserSpec struct {
	Name     string   `json:"name"`
	Policies []string `json:"policies,omitempty"`
}

// TODO: Add status
type UserStatus struct {
}

type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
