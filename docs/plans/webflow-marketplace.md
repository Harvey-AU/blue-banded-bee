# Webflow Marketplace App Submission

Based on official Webflow documentation as of January 2026.

## 1. Current Submission Requirements

### Technical Prerequisites

- **Two-factor authentication** enabled for an admin account on the workspace
- App must be **thoroughly tested and fully functional** with no crashes or
  persistent bugs
- **Clear documentation and error handling** throughout the application
- **Adherence to Webflow's security best practices and privacy guidelines**
- App must be **registered to a workspace connected to a public Webflow
  profile**
- **Only use official Webflow APIs** (no external packages that manipulate
  Webflow)
- **Backend services and APIs must be fully operational** during review

### Code Quality Standards

- Source code must be **well-organised and follow industry standards**
- Code should be **readable, maintainable, and free from unnecessary
  complexity**
- For Designer Extensions: avoid vulnerable patterns like `eval()` statements,
  direct DOM manipulation, and excessive global variables
- External iframes restricted to authentication only

## 2. Required Documentation & Assets

### Visual Assets

- **App Avatar/Logo**: 512×512px (1:1 aspect ratio) OR 900×900px for marketplace
  listing
- **Publisher Logo**: 20×20px, must be recognisable at small sizes
- **App Screenshots**: Minimum 3-5 images at 1280×846px resolution (4+
  recommended)
- **Demo Video**: 1-2 minutes (optional but recommended)
  - If your app uses a Data Client, **must demonstrate working OAuth flow**
    showing users approving and denying requests
  - Should describe deep integration with Webflow

### Written Content

- **App Name**: Maximum 30 characters
- **Short Description**: Maximum 100 characters focusing on core value
- **Long Description**: Maximum 10,000 characters (supports Markdown)
  - Should begin with 2-3 sentence overview
  - Detail capabilities and include setup requirements
  - Reference documentation
- **Feature List**: Up to 5 key capabilities
- **Categories**: Select up to 5 from 19 available options (AI, Analytics,
  Ecommerce, Marketing, SEO, etc.)

### Required URLs

- Valid **website**
- **Privacy policy**
- **Terms of service**
- **Support email**

## 3. Review & Approval Process

### Submission Process

1. Complete thorough testing and bug fixes
2. Prepare all required assets and documentation
3. Submit through the
   [Webflow App submission form](https://developers.webflow.com/submit)
4. Provide complete access for testing (demo account, credentials, or
   fully-featured demo mode)
5. Ensure backend services remain live throughout review

### Review Timeline

- Expect decisions within **10-15 business days**
- Notification sent via registered email
- Incomplete submissions slow the approval process

### Review Criteria

Webflow evaluates apps based on:

- Security and safety compliance
- Technical performance and efficiency
- User experience and usability
- Design consistency
- Legal and intellectual property compliance
- Privacy and data protection standards

## 4. Technical Requirements (API Compliance & Security)

### OAuth & Authentication

- Apps using Data Client must implement **OAuth 2.0** properly
- Must include **CSRF protection** using state tokens
- **Scopes in Install/OAuth URL must match or be subset of app settings scopes**
- Authorisation code can only be used **once**
- **HTTPS required** for all communication (use ngrok, Cloudflare Tunnel, or VS
  Code Tunnels for local development)

### Token Management

- **Never store tokens in source code**
- Store access tokens securely in database or environment variables
- **Regularly rotate API tokens**
- **Revoke compromised tokens immediately**
- Implement robust authentication (OAuth 2.0 or JWT)

### Security Best Practices

- Validate all API requests
- No `eval()` statements or direct DOM manipulation
- Transparent data collection disclosure
- Follow WCAG accessibility standards (alt text, keyboard navigation, colour
  contrast)
- Efficient resource usage and smooth responsiveness

### Prohibited Practices

- Malicious content or security compromise attempts
- Displaying advertisements to users
- Hidden fees or deceptive pricing
- Unauthorised trademark or logo use
- Privacy infringement or data misuse

## 5. Listing Requirements

### Content Policies

- **No offensive, insensitive, or disgusting content**
- Must fully adhere to Webflow's Acceptable Use Policy
- Ensure you have **proper rights, licences, and permissions** for all content
- No intellectual property infringement
- Comply with applicable laws across all jurisdictions

### Business Requirements

- **Clear pricing information** (no hidden fees)
- **No advertising** to maintain distraction-free experience
- **Single developer account** per marketplace listing (no multiple accounts)
- Disclose all affiliations transparently
- Active maintenance and support

### User Experience Standards

- Comprehensive help resources and support materials
- Consistent visual design and user interface
- Action-oriented language in descriptions
- Clear documentation for non-obvious features and in-app purchases
- Error-prone or poorly maintained apps will be removed

### Updates

Maintain current listings through the app submission form by selecting "App
Update" as submission type. For updates, only **App Name and Client ID are
required**; all other fields are optional.

## Sources

- [Submitting Your App to the Webflow Marketplace | Webflow Developer Documentation](https://developers.webflow.com/data/docs/submitting-your-app)
- [Marketplace Guidelines | Webflow Developer Documentation](https://developers.webflow.com/data/v2.0.0-beta/apps/docs/marketplace-guidelines)
- [Listing your App | Webflow Developer Documentation](https://developers.webflow.com/apps/docs/marketplace/listing-your-app)
- [Submit a Webflow App | Webflow](https://developers.webflow.com/submit)
- [OAuth | Webflow Developer Documentation](https://developers.webflow.com/data/reference/oauth-app)
- [Essential security practices for building a Webflow App | Webflow Blog](https://webflow.com/blog/essential-security-practices-building-webflow-app)
- [Authenticating with the Webflow API](https://developers.webflow.com/data/reference/authentication)
