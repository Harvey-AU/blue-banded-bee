/**
 * Authentication helpers for MCP Playwright testing
 */

// Load test credentials
const TEST_EMAIL = process.env.TEST_EMAIL || 'test@bluebandedbee.dev';
const TEST_PASSWORD = process.env.TEST_PASSWORD || 'testpass123';
const DASHBOARD_URL = process.env.DASHBOARD_URL || 'https://app.bluebandedbee.co/dashboard';

/**
 * Create a test account via the signup modal
 */
async function createTestAccount(page) {
  console.log('Creating test account...');
  
  // Navigate to dashboard
  await page.goto(DASHBOARD_URL);
  
  // Wait for page load and click Sign In button
  await page.waitForSelector('#loginBtn');
  await page.click('#loginBtn');
  
  // Wait for auth modal to appear
  await page.waitForSelector('#authModal.show');
  
  // Switch to signup form
  await page.click('[bb-action="show-signup"]');
  await page.waitForSelector('#signupForm[style*="block"]');
  
  // Fill signup form
  await page.fill('#signupEmail', TEST_EMAIL);
  await page.fill('#signupPassword', TEST_PASSWORD);
  await page.fill('#signupPasswordConfirm', TEST_PASSWORD);
  
  // Submit signup form
  await page.click('#signupForm button[type="submit"]');
  
  // Wait for either success or error
  try {
    // Wait for success (modal closes) or error message
    await Promise.race([
      page.waitForSelector('#authModal:not(.show)', { timeout: 5000 }),
      page.waitForSelector('#authError[style*="block"]', { timeout: 5000 })
    ]);
    
    // Check if there's an error
    const errorElement = await page.$('#authError[style*="block"]');
    if (errorElement) {
      const errorText = await errorElement.textContent();
      console.log('Signup result:', errorText);
      
      if (errorText.includes('confirmation link')) {
        console.log('Account created, needs email confirmation');
        return { success: true, needsConfirmation: true };
      } else if (errorText.includes('already registered')) {
        console.log('Account already exists, proceeding to login');
        return { success: true, alreadyExists: true };
      } else {
        throw new Error(`Signup failed: ${errorText}`);
      }
    }
    
    console.log('Account created successfully');
    return { success: true };
    
  } catch (error) {
    console.error('Account creation failed:', error.message);
    throw error;
  }
}

/**
 * Login with test account credentials
 */
async function loginWithTestAccount(page) {
  console.log('Logging in with test account...');
  
  // Navigate to dashboard if not already there
  const currentUrl = page.url();
  if (!currentUrl.includes('bluebandedbee.co')) {
    await page.goto(DASHBOARD_URL);
  }
  
  // Check if already logged in
  const userInfo = await page.$('[data-bb-auth="required"]');
  if (userInfo && await userInfo.isVisible()) {
    console.log('Already logged in');
    return { success: true, alreadyLoggedIn: true };
  }
  
  // Click Sign In button
  await page.waitForSelector('#loginBtn');
  await page.click('#loginBtn');
  
  // Wait for auth modal
  await page.waitForSelector('#authModal.show');
  
  // Make sure we're on login form (not signup)
  const loginForm = await page.$('#loginForm[style*="block"]');
  if (!loginForm) {
    await page.click('[bb-action="show-login"]');
    await page.waitForSelector('#loginForm[style*="block"]');
  }
  
  // Fill login form
  await page.fill('#loginEmail', TEST_EMAIL);
  await page.fill('#loginPassword', TEST_PASSWORD);
  
  // Submit login form
  await page.click('#emailLoginForm button[type="submit"]');
  
  // Wait for login result
  try {
    await Promise.race([
      page.waitForSelector('#authModal:not(.show)', { timeout: 10000 }),
      page.waitForSelector('#authError[style*="block"]', { timeout: 5000 })
    ]);
    
    // Check for errors
    const errorElement = await page.$('#authError[style*="block"]');
    if (errorElement) {
      const errorText = await errorElement.textContent();
      throw new Error(`Login failed: ${errorText}`);
    }
    
    // Wait for page reload and verify login
    await page.waitForLoadState('networkidle');
    await page.waitForSelector('[data-bb-auth="required"]', { timeout: 5000 });
    
    console.log('Login successful');
    return { success: true };
    
  } catch (error) {
    console.error('Login failed:', error.message);
    throw error;
  }
}

/**
 * Logout from current session
 */
async function logoutTestAccount(page) {
  console.log('Logging out...');
  
  try {
    // Check if logout button is visible
    const logoutBtn = await page.$('#logoutBtn');
    if (logoutBtn && await logoutBtn.isVisible()) {
      await logoutBtn.click();
      
      // Wait for page reload
      await page.waitForLoadState('networkidle');
      
      // Verify logout by checking for login button
      await page.waitForSelector('#loginBtn', { timeout: 5000 });
      
      console.log('Logout successful');
      return { success: true };
    } else {
      console.log('Not logged in, no logout needed');
      return { success: true, notLoggedIn: true };
    }
    
  } catch (error) {
    console.error('Logout failed:', error.message);
    throw error;
  }
}

/**
 * Setup authenticated test session
 * Creates account if needed, then logs in
 */
async function setupAuthenticatedSession(page) {
  console.log('Setting up authenticated test session...');
  
  try {
    // Try to login first (account might already exist)
    try {
      const loginResult = await loginWithTestAccount(page);
      if (loginResult.success && !loginResult.alreadyLoggedIn) {
        console.log('Login successful');
        return { success: true, action: 'logged_in' };
      }
      if (loginResult.alreadyLoggedIn) {
        console.log('Already authenticated');
        return { success: true, action: 'already_authenticated' };
      }
    } catch (loginError) {
      console.log('Login failed, attempting to create account...');
      
      // If login fails, try to create account
      const signupResult = await createTestAccount(page);
      
      if (signupResult.success && signupResult.needsConfirmation) {
        throw new Error('Account created but needs email confirmation. Please check email and confirm before testing.');
      }
      
      if (signupResult.success && signupResult.alreadyExists) {
        throw new Error('Account exists but login failed. Please check credentials.');
      }
      
      if (signupResult.success) {
        console.log('Account created, attempting login...');
        const loginResult = await loginWithTestAccount(page);
        if (loginResult.success) {
          return { success: true, action: 'account_created_and_logged_in' };
        }
      }
    }
    
    throw new Error('Failed to setup authenticated session');
    
  } catch (error) {
    console.error('Authentication setup failed:', error.message);
    throw error;
  }
}

module.exports = {
  TEST_EMAIL,
  TEST_PASSWORD,
  DASHBOARD_URL,
  createTestAccount,
  loginWithTestAccount,
  logoutTestAccount,
  setupAuthenticatedSession
};