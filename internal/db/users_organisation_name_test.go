package db

import (
	"testing"
)

func TestDeriveOrganisationName(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		fullName *string
		expected string
	}{
		// Business email tests
		{
			name:     "business_email_teamharvey_co",
			email:    "simon@teamharvey.co",
			fullName: stringPtr("Simon Small-Chua"),
			expected: "Teamharvey",
		},
		{
			name:     "business_email_with_com",
			email:    "user@acme.com",
			fullName: nil,
			expected: "Acme",
		},
		{
			name:     "business_email_with_com_au",
			email:    "user@example.com.au",
			fullName: nil,
			expected: "Example",
		},
		{
			name:     "business_email_with_co_uk",
			email:    "user@company.co.uk",
			fullName: nil,
			expected: "Company",
		},
		{
			name:     "business_email_with_io",
			email:    "user@startup.io",
			fullName: nil,
			expected: "Startup",
		},
		{
			name:     "business_email_with_ai",
			email:    "user@aicompany.ai",
			fullName: nil,
			expected: "Aicompany",
		},
		{
			name:     "business_email_with_dev",
			email:    "user@devops.dev",
			fullName: nil,
			expected: "Devops",
		},

		// Personal email tests - Gmail
		{
			name:     "gmail_with_fullname",
			email:    "simon.smallchua@gmail.com",
			fullName: stringPtr("Simon Small-Chua"),
			expected: "Simon Small-Chua",
		},
		{
			name:     "gmail_without_fullname",
			email:    "user@gmail.com",
			fullName: nil,
			expected: "User Organisation",
		},
		{
			name:     "gmail_with_empty_fullname",
			email:    "user@gmail.com",
			fullName: stringPtr(""),
			expected: "User Organisation",
		},
		{
			name:     "gmail_dotted_username_without_fullname",
			email:    "simon.smallchua@gmail.com",
			fullName: nil,
			expected: "Simon.Smallchua Organisation",
		},
		{
			name:     "googlemail_with_fullname",
			email:    "user@googlemail.com",
			fullName: stringPtr("John Doe"),
			expected: "John Doe",
		},

		// Personal email tests - Outlook/Hotmail/Live
		{
			name:     "outlook_with_fullname",
			email:    "user@outlook.com",
			fullName: stringPtr("Jane Smith"),
			expected: "Jane Smith",
		},
		{
			name:     "hotmail_without_fullname",
			email:    "user@hotmail.com",
			fullName: nil,
			expected: "User Organisation",
		},
		{
			name:     "live_with_fullname",
			email:    "user@live.com",
			fullName: stringPtr("Bob Wilson"),
			expected: "Bob Wilson",
		},

		// Personal email tests - Yahoo
		{
			name:     "yahoo_with_fullname",
			email:    "user@yahoo.com",
			fullName: stringPtr("Alice Brown"),
			expected: "Alice Brown",
		},
		{
			name:     "ymail_without_fullname",
			email:    "user@ymail.com",
			fullName: nil,
			expected: "User Organisation",
		},

		// Personal email tests - iCloud
		{
			name:     "icloud_with_fullname",
			email:    "user@icloud.com",
			fullName: stringPtr("Charlie Davis"),
			expected: "Charlie Davis",
		},
		{
			name:     "me_com_with_fullname",
			email:    "user@me.com",
			fullName: stringPtr("David Evans"),
			expected: "David Evans",
		},
		{
			name:     "mac_com_without_fullname",
			email:    "user@mac.com",
			fullName: nil,
			expected: "User Organisation",
		},

		// Personal email tests - Other providers
		{
			name:     "protonmail_with_fullname",
			email:    "user@protonmail.com",
			fullName: stringPtr("Eve Foster"),
			expected: "Eve Foster",
		},
		{
			name:     "proton_me_without_fullname",
			email:    "user@proton.me",
			fullName: nil,
			expected: "User Organisation",
		},
		{
			name:     "aol_with_fullname",
			email:    "user@aol.com",
			fullName: stringPtr("Frank Green"),
			expected: "Frank Green",
		},
		{
			name:     "zoho_without_fullname",
			email:    "user@zoho.com",
			fullName: nil,
			expected: "User Organisation",
		},
		{
			name:     "fastmail_with_fullname",
			email:    "user@fastmail.com",
			fullName: stringPtr("Grace Hill"),
			expected: "Grace Hill",
		},

		// Edge cases
		{
			name:     "invalid_email_no_at_with_fullname",
			email:    "notanemail",
			fullName: stringPtr("Henry Jones"),
			expected: "Henry Jones",
		},
		{
			name:     "invalid_email_no_at_without_fullname",
			email:    "notanemail",
			fullName: nil,
			expected: "Personal Organisation",
		},
		{
			name:     "empty_domain_after_at",
			email:    "user@",
			fullName: stringPtr("Irene King"),
			expected: "Irene King",
		},
		{
			name:     "business_email_co_nz",
			email:    "user@company.co.nz",
			fullName: nil,
			expected: "Company",
		},
		{
			name:     "business_email_net",
			email:    "user@network.net",
			fullName: nil,
			expected: "Network",
		},
		{
			name:     "business_email_org",
			email:    "user@nonprofit.org",
			fullName: nil,
			expected: "Nonprofit",
		},
		{
			name:     "business_email_no_recognised_tld",
			email:    "user@company.xyz",
			fullName: nil,
			expected: "Company.xyz",
		},
		{
			name:     "uppercase_business_email",
			email:    "User@TeamHarvey.CO",
			fullName: nil,
			expected: "Teamharvey",
		},
		{
			name:     "uppercase_gmail",
			email:    "User@GMAIL.COM",
			fullName: stringPtr("Jack Lewis"),
			expected: "Jack Lewis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveOrganisationName(tt.email, tt.fullName)
			if result != tt.expected {
				t.Errorf("deriveOrganisationName(%q, %v) = %q, want %q",
					tt.email, formatFullName(tt.fullName), result, tt.expected)
			}
		})
	}
}

// Helper function for pointer creation
func stringPtr(s string) *string {
	return &s
}

// Helper function for readable test output
func formatFullName(s *string) string {
	if s == nil {
		return "nil"
	}
	if *s == "" {
		return `""`
	}
	return `"` + *s + `"`
}
