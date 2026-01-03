-- Simplified seed for Blue Banded Bee
-- Contains only essential reference data, no transient job/task/page data
SET session_replication_role = replica;  -- Disable triggers for fast bulk load
SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET search_path = public, auth;
SET check_function_bodies = false;
SET row_security = off;

--
-- Data for Name: users; Type: TABLE DATA; Schema: auth; Owner: -
--
INSERT INTO auth.users (instance_id, id, aud, role, email, encrypted_password, email_confirmed_at, invited_at, confirmation_token, confirmation_sent_at, recovery_token, recovery_sent_at, email_change_token_new, email_change, email_change_sent_at, last_sign_in_at, raw_app_meta_data, raw_user_meta_data, is_super_admin, created_at, updated_at, phone, phone_confirmed_at, phone_change, phone_change_token, phone_change_sent_at, email_change_token_current, email_change_confirm_status, banned_until, reauthentication_token, reauthentication_sent_at, is_sso_user, deleted_at, is_anonymous)
VALUES
    ('00000000-0000-0000-0000-000000000000', '64d361fa-23fc-4deb-8a1b-3016a6c2e339', 'authenticated', 'authenticated', 'simon@teamharvey.co', NULL, '2025-08-06 10:21:07.694503+00', NULL, '', NULL, '', NULL, '', '', NULL, '2025-12-28 12:13:06.423926+00', '{"provider": "google", "providers": ["google"], "system_role": "system_admin"}', '{"iss": "https://accounts.google.com", "sub": "115865462408113540093", "name": "Simon Smallchua", "email": "simon@teamharvey.co", "picture": "https://lh3.googleusercontent.com/a/ACg8ocIiHA2rNRKbUhEc6gSMpfXqyCt5j1S1_3ENKC6UUsdypqIXgp2v=s96-c", "full_name": "Simon Smallchua", "avatar_url": "https://lh3.googleusercontent.com/a/ACg8ocIiHA2rNRKbUhEc6gSMpfXqyCt5j1S1_3ENKC6UUsdypqIXgp2v=s96-c", "provider_id": "115865462408113540093", "custom_claims": {"hd": "teamharvey.co"}, "email_verified": true, "phone_verified": false}', NULL, '2025-08-06 10:21:07.676382+00', '2025-12-28 12:13:56.329528+00', NULL, NULL, '', '', NULL, DEFAULT, '', 0, NULL, '', NULL, false, NULL, false),
    ('00000000-0000-0000-0000-000000000000', 'd736c69b-6597-48d1-adbb-bcfb57be550e', 'authenticated', 'authenticated', 'simon.smallchua@gmail.com', NULL, '2025-10-09 08:11:06.799448+00', NULL, '', NULL, '', NULL, '', '', NULL, '2025-10-09 08:11:06.805582+00', '{"provider": "google", "providers": ["google"]}', '{"iss": "https://accounts.google.com", "sub": "116532901977231133800", "name": "Simon Smallchua", "email": "simon.smallchua@gmail.com", "picture": "https://lh3.googleusercontent.com/a/ACg8ocLcKp5gWQczA2vP6J9DG6gsB6C6ir5ON_KXI8Qs3HBu-6CDPErp=s96-c", "full_name": "Simon Smallchua", "avatar_url": "https://lh3.googleusercontent.com/a/ACg8ocLcKp5gWQczA2vP6J9DG6gsB6C6ir5ON_KXI8Qs3HBu-6CDPErp=s96-c", "provider_id": "116532901977231133800", "email_verified": true, "phone_verified": false}', NULL, '2025-10-09 08:11:06.76954+00', '2025-10-09 09:52:04.166523+00', NULL, NULL, '', '', NULL, DEFAULT, '', 0, NULL, '', NULL, false, NULL, false);

--
-- Data for Name: identities; Type: TABLE DATA; Schema: auth; Owner: -
-- Note: email column is GENERATED ALWAYS from identity_data, so we omit it
--
INSERT INTO auth.identities (provider_id, user_id, identity_data, provider, last_sign_in_at, created_at, updated_at, id)
VALUES
    ('116532901977231133800', 'd736c69b-6597-48d1-adbb-bcfb57be550e', '{"iss": "https://accounts.google.com", "sub": "116532901977231133800", "name": "Simon Smallchua", "email": "simon.smallchua@gmail.com", "picture": "https://lh3.googleusercontent.com/a/ACg8ocLcKp5gWQczA2vP6J9DG6gsB6C6ir5ON_KXI8Qs3HBu-6CDPErp=s96-c", "full_name": "Simon Smallchua", "avatar_url": "https://lh3.googleusercontent.com/a/ACg8ocLcKp5gWQczA2vP6J9DG6gsB6C6ir5ON_KXI8Qs3HBu-6CDPErp=s96-c", "provider_id": "116532901977231133800", "email_verified": true, "phone_verified": false}', 'google', '2025-10-09 08:11:06.788968+00', '2025-10-09 08:11:06.789025+00', '2025-10-09 08:11:06.789025+00', 'eb4a6be9-7f6a-4239-97c6-67c32006166f'),
    ('115865462408113540093', '64d361fa-23fc-4deb-8a1b-3016a6c2e339', '{"iss": "https://accounts.google.com", "sub": "115865462408113540093", "name": "Simon Smallchua", "email": "simon@teamharvey.co", "picture": "https://lh3.googleusercontent.com/a/ACg8ocIiHA2rNRKbUhEc6gSMpfXqyCt5j1S1_3ENKC6UUsdypqIXgp2v=s96-c", "full_name": "Simon Smallchua", "avatar_url": "https://lh3.googleusercontent.com/a/ACg8ocIiHA2rNRKbUhEc6gSMpfXqyCt5j1S1_3ENKC6UUsdypqIXgp2v=s96-c", "provider_id": "115865462408113540093", "custom_claims": {"hd": "teamharvey.co"}, "email_verified": true, "phone_verified": false}', 'google', '2025-08-06 10:21:07.690062+00', '2025-08-06 10:21:07.69012+00', '2025-12-28 12:13:06.418241+00', 'd3f737a6-359e-4375-9b94-67117c8dc963');

--
-- Data for Name: organisations; Type: TABLE DATA; Schema: public; Owner: -
--
INSERT INTO public.organisations (id, name, created_at, updated_at)
VALUES
    ('96f7546c-47ea-41f8-a3a3-46b4deb84105', 'Personal Organisation', '2025-11-02 00:11:21.520651+00', '2025-11-02 00:11:21.520651+00'),
    ('a1b2c3d4-e5f6-4a5b-8c9d-1234567890ab', 'Harvey', '2025-11-15 00:00:00+00', '2025-11-15 00:00:00+00'),
    ('b2c3d4e5-f6a7-5b6c-9d0e-234567890abc', 'Merry People', '2025-11-20 00:00:00+00', '2025-11-20 00:00:00+00');

--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--
INSERT INTO public.users (id, email, full_name, organisation_id, created_at, updated_at, active_organisation_id)
VALUES
    ('64d361fa-23fc-4deb-8a1b-3016a6c2e339', 'simon@teamharvey.co', 'Simon Smallchua', '96f7546c-47ea-41f8-a3a3-46b4deb84105', '2025-11-02 00:11:21.520651+00', '2025-11-02 00:11:21.520651+00', '96f7546c-47ea-41f8-a3a3-46b4deb84105');

--
-- Data for Name: organisation_members; Type: TABLE DATA; Schema: public; Owner: -
--
INSERT INTO public.organisation_members (user_id, organisation_id, created_at)
VALUES
    ('64d361fa-23fc-4deb-8a1b-3016a6c2e339', '96f7546c-47ea-41f8-a3a3-46b4deb84105', '2025-11-02 00:11:21.520651+00'),
    ('64d361fa-23fc-4deb-8a1b-3016a6c2e339', 'a1b2c3d4-e5f6-4a5b-8c9d-1234567890ab', '2025-11-15 00:00:00+00');

--
-- Data for Name: domains; Type: TABLE DATA; Schema: public; Owner: -
--
INSERT INTO public.domains (id, name, crawl_delay_seconds, adaptive_delay_seconds, adaptive_delay_floor_seconds, created_at)
VALUES
    (1, 'teamharvey.co', NULL, 0, 0, '2025-12-28 10:49:41.041287+00'),
    (2, 'cpsn.org.au', NULL, 0, 0, '2025-12-28 10:58:38.844544+00'),
    (3, 'envirotecture.com.au', NULL, 0, 0, '2025-12-28 11:00:00+00');

-- Reset domain sequence to avoid conflicts
SELECT setval('domains_id_seq', (SELECT MAX(id) FROM domains));

--
-- Data for Name: schedulers; Type: TABLE DATA; Schema: public; Owner: -
--
INSERT INTO public.schedulers (id, domain_id, organisation_id, schedule_interval_hours, next_run_at, is_enabled, concurrency, find_links, max_pages, include_paths, exclude_paths, required_workers, created_at, updated_at)
VALUES
    ('14dd9d7a-2696-4479-831c-e43163795e36', 1, '96f7546c-47ea-41f8-a3a3-46b4deb84105', 12, NOW() + INTERVAL '12 hours', true, 20, true, 0, NULL, NULL, 1, NOW(), NOW()),
    ('4db618ce-5b05-409f-8a06-fdf4a4a9745c', 2, '96f7546c-47ea-41f8-a3a3-46b4deb84105', 12, NOW() + INTERVAL '12 hours', true, 20, true, 0, NULL, NULL, 1, NOW(), NOW()),
    ('5ec729df-6c16-5100-9b17-ae05b5ba856d', 3, '96f7546c-47ea-41f8-a3a3-46b4deb84105', 24, NOW() + INTERVAL '24 hours', true, 20, true, 0, NULL, NULL, 1, NOW(), NOW());

-- Re-enable triggers
SET session_replication_role = DEFAULT;
