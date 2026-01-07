-- Reduce free plan quota to 600 for testing threshold behaviour
UPDATE plans SET daily_page_limit = 600 WHERE name = 'free';
