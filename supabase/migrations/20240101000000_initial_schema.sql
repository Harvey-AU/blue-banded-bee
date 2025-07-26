

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE SCHEMA IF NOT EXISTS "public";


ALTER SCHEMA "public" OWNER TO "pg_database_owner";


COMMENT ON SCHEMA "public" IS 'standard public schema';



CREATE OR REPLACE FUNCTION "public"."recalculate_job_stats"("job_id" "text") RETURNS "void"
    LANGUAGE "plpgsql"
    AS $$
  BEGIN
      UPDATE jobs j
      SET
          total_tasks = t.total_count,
          sitemap_tasks = t.sitemap_count,
          found_tasks = t.found_count,
          completed_tasks = t.completed_count,
          failed_tasks = t.failed_count,
          skipped_tasks = t.skipped_count,
          progress = CASE
              WHEN t.total_count > 0 AND (t.total_count - t.skipped_count) > 0
              THEN (t.completed_count + t.failed_count)::REAL / (t.total_count - t.skipped_count)::REAL * 100.0
              ELSE 0.0
          END
      FROM (
          SELECT
              COUNT(*) as total_count,
              COUNT(*) FILTER (WHERE source_type = 'sitemap') as sitemap_count,
              COUNT(*) FILTER (WHERE source_type != 'sitemap' OR source_type IS NULL) as found_count,
              COUNT(*) FILTER (WHERE status = 'completed') as completed_count,
              COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
              COUNT(*) FILTER (WHERE status = 'skipped') as skipped_count
          FROM tasks
          WHERE tasks.job_id = recalculate_job_stats.job_id
      ) t
      WHERE j.id = job_id;
  END;
  $$;


ALTER FUNCTION "public"."recalculate_job_stats"("job_id" "text") OWNER TO "postgres";


CREATE OR REPLACE FUNCTION "public"."set_job_completed_at"() RETURNS "trigger"
    LANGUAGE "plpgsql"
    AS $$
		BEGIN
		  -- Set completed_at when progress reaches 100% and it's not already set
		  -- Handle both INSERT and UPDATE operations
		  IF NEW.progress >= 100.0 AND (TG_OP = 'INSERT' OR OLD.completed_at IS NULL) AND NEW.completed_at IS NULL THEN
		    NEW.completed_at = NOW();
		  END IF;
		  
		  RETURN NEW;
		END;
		$$;


ALTER FUNCTION "public"."set_job_completed_at"() OWNER TO "postgres";


CREATE OR REPLACE FUNCTION "public"."set_job_started_at"() RETURNS "trigger"
    LANGUAGE "plpgsql"
    AS $$
		BEGIN
		  -- Only set started_at if it's currently NULL and completed_tasks > 0
		  -- Handle both INSERT and UPDATE operations
		  IF NEW.completed_tasks > 0 AND (TG_OP = 'INSERT' OR OLD.started_at IS NULL) AND NEW.started_at IS NULL THEN
		    NEW.started_at = NOW();
		  END IF;
		  
		  RETURN NEW;
		END;
		$$;


ALTER FUNCTION "public"."set_job_started_at"() OWNER TO "postgres";


CREATE OR REPLACE FUNCTION "public"."update_job_progress"() RETURNS "trigger"
    LANGUAGE "plpgsql"
    AS $$
		DECLARE
		    job_id_to_update TEXT;
		    total_tasks INTEGER;
		    completed_count INTEGER;
		    failed_count INTEGER;
		    skipped_count INTEGER;
		    new_progress REAL;
		BEGIN
		    -- Determine which job to update
		    IF TG_OP = 'DELETE' THEN
		        job_id_to_update = OLD.job_id;
		    ELSE
		        job_id_to_update = NEW.job_id;
		    END IF;
		    
		    -- Get the total tasks for this job
		    SELECT j.total_tasks INTO total_tasks
		    FROM jobs j
		    WHERE j.id = job_id_to_update;
		    
		    -- Count completed, failed, and skipped tasks
		    SELECT 
		        COUNT(*) FILTER (WHERE status = 'completed'),
		        COUNT(*) FILTER (WHERE status = 'failed'),
		        COUNT(*) FILTER (WHERE status = 'skipped')
		    INTO completed_count, failed_count, skipped_count
		    FROM tasks
		    WHERE job_id = job_id_to_update;
		    
		    -- Calculate progress percentage (only count completed + failed, not skipped)
		    IF total_tasks > 0 AND (total_tasks - skipped_count) > 0 THEN
		        new_progress = (completed_count + failed_count)::REAL / (total_tasks - skipped_count)::REAL * 100.0;
		    ELSE
		        new_progress = 0.0;
		    END IF;
		    
		    -- Update the job with new counts and progress
		    UPDATE jobs
		    SET 
		        completed_tasks = completed_count,
		        failed_tasks = failed_count,
		        skipped_tasks = skipped_count,
		        progress = new_progress,
		        status = CASE 
		            WHEN new_progress >= 100.0 THEN 'completed'
		            WHEN completed_count > 0 OR failed_count > 0 THEN 'running'
		            ELSE status
		        END
		    WHERE id = job_id_to_update;
		    
		    -- Return the appropriate record based on operation
		    IF TG_OP = 'DELETE' THEN
		        RETURN OLD;
		    ELSE
		        RETURN NEW;
		    END IF;
		END;
		$$;


ALTER FUNCTION "public"."update_job_progress"() OWNER TO "postgres";

SET default_tablespace = '';

SET default_table_access_method = "heap";


CREATE TABLE IF NOT EXISTS "public"."domains" (
    "id" integer NOT NULL,
    "name" "text" NOT NULL,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL
);


ALTER TABLE "public"."domains" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."domains_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE "public"."domains_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."domains_id_seq" OWNED BY "public"."domains"."id";



CREATE TABLE IF NOT EXISTS "public"."jobs" (
    "id" "text" NOT NULL,
    "domain_id" integer NOT NULL,
    "status" "text" NOT NULL,
    "progress" real NOT NULL,
    "sitemap_tasks" integer DEFAULT 0 NOT NULL,
    "found_tasks" integer DEFAULT 0 NOT NULL,
    "total_tasks" integer DEFAULT 0 NOT NULL,
    "completed_tasks" integer DEFAULT 0 NOT NULL,
    "failed_tasks" integer DEFAULT 0 NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    "started_at" timestamp without time zone,
    "completed_at" timestamp without time zone,
    "concurrency" integer NOT NULL,
    "find_links" boolean NOT NULL,
    "max_pages" integer NOT NULL,
    "include_paths" "text",
    "exclude_paths" "text",
    "required_workers" integer DEFAULT 0,
    "skipped_tasks" integer DEFAULT 0,
    "user_id" "uuid",
    "organisation_id" "uuid",
    "error_message" "text",
    "source_type" character varying(50),
    "source_detail" "text",
    "source_info" "text"
);


ALTER TABLE "public"."jobs" OWNER TO "postgres";


CREATE TABLE IF NOT EXISTS "public"."organisations" (
    "id" "uuid" DEFAULT "gen_random_uuid"() NOT NULL,
    "name" "text" NOT NULL,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT "now"() NOT NULL
);


ALTER TABLE "public"."organisations" OWNER TO "postgres";


CREATE TABLE IF NOT EXISTS "public"."pages" (
    "id" integer NOT NULL,
    "domain_id" integer NOT NULL,
    "path" "text" NOT NULL,
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL
);


ALTER TABLE "public"."pages" OWNER TO "postgres";


CREATE SEQUENCE IF NOT EXISTS "public"."pages_id_seq"
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE "public"."pages_id_seq" OWNER TO "postgres";


ALTER SEQUENCE "public"."pages_id_seq" OWNED BY "public"."pages"."id";



CREATE TABLE IF NOT EXISTS "public"."tasks" (
    "id" "text" NOT NULL,
    "job_id" "text" NOT NULL,
    "page_id" integer NOT NULL,
    "path" "text" NOT NULL,
    "status" "text" NOT NULL,
    "created_at" timestamp without time zone NOT NULL,
    "started_at" timestamp without time zone,
    "completed_at" timestamp without time zone,
    "retry_count" integer NOT NULL,
    "error" "text",
    "source_type" "text" NOT NULL,
    "source_url" "text",
    "status_code" integer,
    "response_time" bigint,
    "cache_status" "text",
    "content_type" "text",
    "second_response_time" bigint,
    "second_cache_status" "text",
    "priority_score" numeric(4,3) DEFAULT 0.000,
    "cache_check_attempts" "jsonb",
    "content_length" bigint,
    "headers" "jsonb",
    "redirect_url" "text",
    "dns_lookup_time" integer,
    "tcp_connection_time" integer,
    "tls_handshake_time" integer,
    "ttfb" integer,
    "content_transfer_time" integer,
    "second_content_length" bigint,
    "second_headers" "jsonb",
    "second_dns_lookup_time" integer,
    "second_tcp_connection_time" integer,
    "second_tls_handshake_time" integer,
    "second_ttfb" integer,
    "second_content_transfer_time" integer
);


ALTER TABLE "public"."tasks" OWNER TO "postgres";


CREATE TABLE IF NOT EXISTS "public"."users" (
    "id" "uuid" NOT NULL,
    "email" "text" NOT NULL,
    "full_name" "text",
    "organisation_id" "uuid",
    "created_at" timestamp without time zone DEFAULT "now"() NOT NULL,
    "updated_at" timestamp without time zone DEFAULT "now"() NOT NULL
);


ALTER TABLE "public"."users" OWNER TO "postgres";


ALTER TABLE ONLY "public"."domains" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."domains_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."pages" ALTER COLUMN "id" SET DEFAULT "nextval"('"public"."pages_id_seq"'::"regclass");



ALTER TABLE ONLY "public"."domains"
    ADD CONSTRAINT "domains_name_key" UNIQUE ("name");



ALTER TABLE ONLY "public"."domains"
    ADD CONSTRAINT "domains_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."jobs"
    ADD CONSTRAINT "jobs_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."organisations"
    ADD CONSTRAINT "organisations_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."pages"
    ADD CONSTRAINT "pages_domain_id_path_key" UNIQUE ("domain_id", "path");



ALTER TABLE ONLY "public"."pages"
    ADD CONSTRAINT "pages_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."tasks"
    ADD CONSTRAINT "tasks_pkey" PRIMARY KEY ("id");



ALTER TABLE ONLY "public"."users"
    ADD CONSTRAINT "users_email_key" UNIQUE ("email");



ALTER TABLE ONLY "public"."users"
    ADD CONSTRAINT "users_pkey" PRIMARY KEY ("id");



CREATE INDEX "idx_jobs_status_completion" ON "public"."jobs" USING "btree" ("status") WHERE ("status" = 'running'::"text");



CREATE INDEX "idx_tasks_job_id" ON "public"."tasks" USING "btree" ("job_id");



CREATE UNIQUE INDEX "idx_tasks_job_page_unique" ON "public"."tasks" USING "btree" ("job_id", "page_id");



CREATE INDEX "idx_tasks_job_priority" ON "public"."tasks" USING "btree" ("job_id", "priority_score");



CREATE INDEX "idx_tasks_job_status_priority" ON "public"."tasks" USING "btree" ("job_id", "status", "priority_score" DESC);



CREATE INDEX "idx_tasks_pending" ON "public"."tasks" USING "btree" ("status", "created_at") WHERE ("status" = 'pending'::"text");



CREATE INDEX "idx_tasks_pending_claim_order" ON "public"."tasks" USING "btree" ("created_at") WHERE ("status" = 'pending'::"text");



CREATE INDEX "idx_tasks_queue" ON "public"."tasks" USING "btree" ("job_id", "status", "priority_score" DESC, "created_at");



CREATE INDEX "idx_tasks_started_at" ON "public"."tasks" USING "btree" ("started_at");



CREATE INDEX "tasks_started_at_idx" ON "public"."tasks" USING "btree" ("started_at");



CREATE INDEX "tasks_status_idx" ON "public"."tasks" USING "btree" ("status");



CREATE OR REPLACE TRIGGER "trigger_set_job_completed" BEFORE INSERT OR UPDATE ON "public"."jobs" FOR EACH ROW EXECUTE FUNCTION "public"."set_job_completed_at"();



CREATE OR REPLACE TRIGGER "trigger_set_job_started" BEFORE INSERT OR UPDATE ON "public"."jobs" FOR EACH ROW EXECUTE FUNCTION "public"."set_job_started_at"();



CREATE OR REPLACE TRIGGER "trigger_update_job_progress" AFTER INSERT OR DELETE OR UPDATE ON "public"."tasks" FOR EACH ROW EXECUTE FUNCTION "public"."update_job_progress"();



ALTER TABLE ONLY "public"."jobs"
    ADD CONSTRAINT "jobs_domain_id_fkey" FOREIGN KEY ("domain_id") REFERENCES "public"."domains"("id");



ALTER TABLE ONLY "public"."jobs"
    ADD CONSTRAINT "jobs_organisation_id_fkey" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations"("id");



ALTER TABLE ONLY "public"."jobs"
    ADD CONSTRAINT "jobs_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id");



ALTER TABLE ONLY "public"."pages"
    ADD CONSTRAINT "pages_domain_id_fkey" FOREIGN KEY ("domain_id") REFERENCES "public"."domains"("id");



ALTER TABLE ONLY "public"."tasks"
    ADD CONSTRAINT "tasks_job_id_fkey" FOREIGN KEY ("job_id") REFERENCES "public"."jobs"("id");



ALTER TABLE ONLY "public"."tasks"
    ADD CONSTRAINT "tasks_page_id_fkey" FOREIGN KEY ("page_id") REFERENCES "public"."pages"("id");



ALTER TABLE ONLY "public"."users"
    ADD CONSTRAINT "users_organisation_id_fkey" FOREIGN KEY ("organisation_id") REFERENCES "public"."organisations"("id");



CREATE POLICY "Allow anonymous read access" ON "public"."domains" FOR SELECT USING (true);



CREATE POLICY "Allow anonymous read access" ON "public"."jobs" FOR SELECT USING (true);



CREATE POLICY "Allow anonymous read access" ON "public"."pages" FOR SELECT USING (true);



CREATE POLICY "Allow anonymous read access" ON "public"."tasks" FOR SELECT USING (true);



CREATE POLICY "Organisation members can access jobs" ON "public"."jobs" USING (("organisation_id" IN ( SELECT "users"."organisation_id"
   FROM "public"."users"
  WHERE ("users"."id" = "auth"."uid"()))));



CREATE POLICY "Organisation members can access tasks" ON "public"."tasks" USING (("job_id" IN ( SELECT "jobs"."id"
   FROM "public"."jobs"
  WHERE ("jobs"."organisation_id" IN ( SELECT "users"."organisation_id"
           FROM "public"."users"
          WHERE ("users"."id" = "auth"."uid"()))))));



CREATE POLICY "Users can access own data" ON "public"."users" USING (("auth"."uid"() = "id"));



CREATE POLICY "Users can access own organisation" ON "public"."organisations" USING (("id" IN ( SELECT "users"."organisation_id"
   FROM "public"."users"
  WHERE ("users"."id" = "auth"."uid"()))));



ALTER TABLE "public"."domains" ENABLE ROW LEVEL SECURITY;


ALTER TABLE "public"."jobs" ENABLE ROW LEVEL SECURITY;


ALTER TABLE "public"."organisations" ENABLE ROW LEVEL SECURITY;


ALTER TABLE "public"."pages" ENABLE ROW LEVEL SECURITY;


ALTER TABLE "public"."tasks" ENABLE ROW LEVEL SECURITY;


ALTER TABLE "public"."users" ENABLE ROW LEVEL SECURITY;


GRANT USAGE ON SCHEMA "public" TO "postgres";
GRANT USAGE ON SCHEMA "public" TO "anon";
GRANT USAGE ON SCHEMA "public" TO "authenticated";
GRANT USAGE ON SCHEMA "public" TO "service_role";



GRANT ALL ON FUNCTION "public"."recalculate_job_stats"("job_id" "text") TO "anon";
GRANT ALL ON FUNCTION "public"."recalculate_job_stats"("job_id" "text") TO "authenticated";
GRANT ALL ON FUNCTION "public"."recalculate_job_stats"("job_id" "text") TO "service_role";



GRANT ALL ON FUNCTION "public"."set_job_completed_at"() TO "anon";
GRANT ALL ON FUNCTION "public"."set_job_completed_at"() TO "authenticated";
GRANT ALL ON FUNCTION "public"."set_job_completed_at"() TO "service_role";



GRANT ALL ON FUNCTION "public"."set_job_started_at"() TO "anon";
GRANT ALL ON FUNCTION "public"."set_job_started_at"() TO "authenticated";
GRANT ALL ON FUNCTION "public"."set_job_started_at"() TO "service_role";



GRANT ALL ON FUNCTION "public"."update_job_progress"() TO "anon";
GRANT ALL ON FUNCTION "public"."update_job_progress"() TO "authenticated";
GRANT ALL ON FUNCTION "public"."update_job_progress"() TO "service_role";



GRANT ALL ON TABLE "public"."domains" TO "anon";
GRANT ALL ON TABLE "public"."domains" TO "authenticated";
GRANT ALL ON TABLE "public"."domains" TO "service_role";



GRANT ALL ON SEQUENCE "public"."domains_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."domains_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."domains_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."jobs" TO "anon";
GRANT ALL ON TABLE "public"."jobs" TO "authenticated";
GRANT ALL ON TABLE "public"."jobs" TO "service_role";



GRANT ALL ON TABLE "public"."organisations" TO "anon";
GRANT ALL ON TABLE "public"."organisations" TO "authenticated";
GRANT ALL ON TABLE "public"."organisations" TO "service_role";



GRANT ALL ON TABLE "public"."pages" TO "anon";
GRANT ALL ON TABLE "public"."pages" TO "authenticated";
GRANT ALL ON TABLE "public"."pages" TO "service_role";



GRANT ALL ON SEQUENCE "public"."pages_id_seq" TO "anon";
GRANT ALL ON SEQUENCE "public"."pages_id_seq" TO "authenticated";
GRANT ALL ON SEQUENCE "public"."pages_id_seq" TO "service_role";



GRANT ALL ON TABLE "public"."tasks" TO "anon";
GRANT ALL ON TABLE "public"."tasks" TO "authenticated";
GRANT ALL ON TABLE "public"."tasks" TO "service_role";



GRANT ALL ON TABLE "public"."users" TO "anon";
GRANT ALL ON TABLE "public"."users" TO "authenticated";
GRANT ALL ON TABLE "public"."users" TO "service_role";



ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES  TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES  TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES  TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON SEQUENCES  TO "service_role";






ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS  TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS  TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS  TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON FUNCTIONS  TO "service_role";






ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES  TO "postgres";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES  TO "anon";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES  TO "authenticated";
ALTER DEFAULT PRIVILEGES FOR ROLE "postgres" IN SCHEMA "public" GRANT ALL ON TABLES  TO "service_role";






RESET ALL;
