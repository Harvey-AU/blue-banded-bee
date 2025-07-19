# Export supabase query performance result
[
  {
    "rolname": "postgres",
    "query": "UPDATE tasks\n\t\t\tSET status = $3, started_at = $1\n\t\t\tWHERE id = $2",
    "calls": 266565,
    "total_time": 77102527.6181111,
    "prop_total_time": "31.9%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tcontent_length = $7, headers = $8, redirect_url = $9,\n\t\t\t\t\tdns_lookup_time = $10, tcp_connection_time = $11, tls_handshake_time = $12,\n\t\t\t\t\tttfb = $13, content_transfer_time = $14,\n\t\t\t\t\tsecond_response_time = $15, second_cache_status = $16,\n\t\t\t\t\tsecond_content_length = $17, second_headers = $18,\n\t\t\t\t\tsecond_dns_lookup_time = $19, second_tcp_connection_time = $20,\n\t\t\t\t\tsecond_tls_handshake_time = $21, second_ttfb = $22,\n\t\t\t\t\tsecond_content_transfer_time = $23,\n\t\t\t\t\tretry_count = $24, cache_check_attempts = $25\n\t\t\t\tWHERE id = $26",
    "calls": 37207,
    "total_time": 63722633.6756994,
    "prop_total_time": "26.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE jobs\n\t\t\tSET total_tasks = total_tasks + $1,\n\t\t\t\tskipped_tasks = skipped_tasks + $2\n\t\t\tWHERE id = $3",
    "calls": 90382,
    "total_time": 51624756.1642233,
    "prop_total_time": "21.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 11440436,
    "total_time": 36404808.8562005,
    "prop_total_time": "15.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1\n\t\t\t\tWHERE id = $2",
    "calls": 1638,
    "total_time": 5027459.382934,
    "prop_total_time": "2.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tsecond_response_time = $7, second_cache_status = $8,\n\t\t\t\t\tretry_count = $9, cache_check_attempts = $10\n\t\t\t\tWHERE id = $11",
    "calls": 13213,
    "total_time": 1953713.607124,
    "prop_total_time": "0.8%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 689109,
    "total_time": 970622.612879021,
    "prop_total_time": "0.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO tasks (\n\t\t\t\tid, job_id, page_id, path, status, created_at, retry_count,\n\t\t\t\tsource_type, source_url, priority_score\n\t\t\t) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
    "calls": 205965,
    "total_time": 709190.480755987,
    "prop_total_time": "0.3%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT t.id, t.job_id, t.page_id, p.path, t.retry_count \n\t\t\tFROM tasks t\n\t\t\tJOIN pages p ON t.page_id = p.id\n\t\t\tWHERE status = $1 \n\t\t\tAND started_at < $2",
    "calls": 76706,
    "total_time": 629021.952581001,
    "prop_total_time": "0.3%",
    "index_advisor_result": {
      "has_suggestion": true,
      "startup_cost_before": 2540.54,
      "startup_cost_after": 2540.59,
      "total_cost_before": 27483.43,
      "total_cost_after": 12766.08,
      "index_statements": [
        "CREATE INDEX ON public.tasks USING btree (started_at)"
      ]
    }
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, error = $3, retry_count = $4\n\t\t\t\tWHERE id = $5",
    "calls": 4732,
    "total_time": 598439.623715,
    "prop_total_time": "0.2%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tsecond_response_time = $7, second_cache_status = $8\n\t\t\t\tWHERE id = $9",
    "calls": 175793,
    "total_time": 547366.066530999,
    "prop_total_time": "0.2%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT max_pages, \n\t\t\t\t   COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status != $2), $3)\n\t\t\tFROM jobs WHERE id = $1",
    "calls": 90654,
    "total_time": 333628.688332001,
    "prop_total_time": "0.1%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 528.77,
      "startup_cost_after": 528.77,
      "total_cost_before": 530.99,
      "total_cost_after": 530.99,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks t\n\t\tSET priority_score = $1\n\t\tFROM pages p\n\t\tWHERE t.page_id = p.id\n\t\tAND t.job_id = $2\n\t\tAND p.domain_id = $3\n\t\tAND p.path = ANY($4)\n\t\tAND t.priority_score < $1",
    "calls": 122153,
    "total_time": 278067.413357001,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 151430,
    "total_time": 228553.086374001,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE jobs \n\t\t\t\tSET status = $1, completed_at = NOW()\n\t\t\t\tWHERE (completed_tasks + failed_tasks) >= (total_tasks - COALESCE(skipped_tasks, $2))\n\t\t\t\t  AND status = $3\n\t\t\t\tRETURNING id",
    "calls": 911402,
    "total_time": 182117.871199,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score \n\t\t\tFROM tasks \n\t\t\tWHERE status = $2\n\t\t AND job_id = $1\n\t\t\tORDER BY priority_score DESC, created_at ASC\n\t\t\tLIMIT $3\n\t\t\tFOR UPDATE SKIP LOCKED",
    "calls": 194342,
    "total_time": 174980.741236,
    "prop_total_time": "0.1%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 15.44,
      "startup_cost_after": 15.44,
      "total_cost_before": 38.84,
      "total_cost_after": 38.84,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO tasks (\n\t\t\t\tid, job_id, page_id, path, status, created_at, retry_count,\n\t\t\t\tsource_type, source_url\n\t\t\t) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
    "calls": 120492,
    "total_time": 113661.429161,
    "prop_total_time": "0.0%",
    "index_advisor_result": null
  },
  {
    "rolname": "authenticator",
    "query": "SELECT name FROM pg_timezone_names",
    "calls": 913,
    "total_time": 112445.245508,
    "prop_total_time": "0.0%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 0,
      "startup_cost_after": 0,
      "total_cost_before": 10,
      "total_cost_after": 10,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 36658,
    "total_time": 111746.023833,
    "prop_total_time": "0.0%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT id, job_id, url, depth, created_at, retry_count, source_type, source_url \n\t\t\tFROM tasks \n\t\t\tWHERE status = $1\n\t\t\n\t\t\tORDER BY created_at ASC\n\t\t\tLIMIT $2\n\t\t\tFOR UPDATE SKIP LOCKED",
    "calls": 6198401,
    "total_time": 92599.4945479757,
    "prop_total_time": "0.0%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": null,
      "startup_cost_after": null,
      "total_cost_before": null,
      "total_cost_after": null,
      "index_statements": []
    }
  }
][
  {
    "rolname": "postgres",
    "query": "UPDATE tasks\n\t\t\tSET status = $3, started_at = $1\n\t\t\tWHERE id = $2",
    "calls": 266565,
    "total_time": 77102527.6181111,
    "prop_total_time": "31.9%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tcontent_length = $7, headers = $8, redirect_url = $9,\n\t\t\t\t\tdns_lookup_time = $10, tcp_connection_time = $11, tls_handshake_time = $12,\n\t\t\t\t\tttfb = $13, content_transfer_time = $14,\n\t\t\t\t\tsecond_response_time = $15, second_cache_status = $16,\n\t\t\t\t\tsecond_content_length = $17, second_headers = $18,\n\t\t\t\t\tsecond_dns_lookup_time = $19, second_tcp_connection_time = $20,\n\t\t\t\t\tsecond_tls_handshake_time = $21, second_ttfb = $22,\n\t\t\t\t\tsecond_content_transfer_time = $23,\n\t\t\t\t\tretry_count = $24, cache_check_attempts = $25\n\t\t\t\tWHERE id = $26",
    "calls": 37207,
    "total_time": 63722633.6756994,
    "prop_total_time": "26.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE jobs\n\t\t\tSET total_tasks = total_tasks + $1,\n\t\t\t\tskipped_tasks = skipped_tasks + $2\n\t\t\tWHERE id = $3",
    "calls": 90382,
    "total_time": 51624756.1642233,
    "prop_total_time": "21.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 11440436,
    "total_time": 36404808.8562005,
    "prop_total_time": "15.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1\n\t\t\t\tWHERE id = $2",
    "calls": 1638,
    "total_time": 5027459.382934,
    "prop_total_time": "2.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tsecond_response_time = $7, second_cache_status = $8,\n\t\t\t\t\tretry_count = $9, cache_check_attempts = $10\n\t\t\t\tWHERE id = $11",
    "calls": 13213,
    "total_time": 1953713.607124,
    "prop_total_time": "0.8%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 689109,
    "total_time": 970622.612879021,
    "prop_total_time": "0.4%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO tasks (\n\t\t\t\tid, job_id, page_id, path, status, created_at, retry_count,\n\t\t\t\tsource_type, source_url, priority_score\n\t\t\t) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
    "calls": 205965,
    "total_time": 709190.480755987,
    "prop_total_time": "0.3%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT t.id, t.job_id, t.page_id, p.path, t.retry_count \n\t\t\tFROM tasks t\n\t\t\tJOIN pages p ON t.page_id = p.id\n\t\t\tWHERE status = $1 \n\t\t\tAND started_at < $2",
    "calls": 76706,
    "total_time": 629021.952581001,
    "prop_total_time": "0.3%",
    "index_advisor_result": {
      "has_suggestion": true,
      "startup_cost_before": 2540.54,
      "startup_cost_after": 2540.59,
      "total_cost_before": 27483.43,
      "total_cost_after": 12766.08,
      "index_statements": [
        "CREATE INDEX ON public.tasks USING btree (started_at)"
      ]
    }
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, error = $3, retry_count = $4\n\t\t\t\tWHERE id = $5",
    "calls": 4732,
    "total_time": 598439.623715,
    "prop_total_time": "0.2%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks \n\t\t\t\tSET status = $1, completed_at = $2, status_code = $3, \n\t\t\t\t\tresponse_time = $4, cache_status = $5, content_type = $6,\n\t\t\t\t\tsecond_response_time = $7, second_cache_status = $8\n\t\t\t\tWHERE id = $9",
    "calls": 175793,
    "total_time": 547366.066530999,
    "prop_total_time": "0.2%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT max_pages, \n\t\t\t\t   COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status != $2), $3)\n\t\t\tFROM jobs WHERE id = $1",
    "calls": 90654,
    "total_time": 333628.688332001,
    "prop_total_time": "0.1%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 528.77,
      "startup_cost_after": 528.77,
      "total_cost_before": 530.99,
      "total_cost_after": 530.99,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "UPDATE tasks t\n\t\tSET priority_score = $1\n\t\tFROM pages p\n\t\tWHERE t.page_id = p.id\n\t\tAND t.job_id = $2\n\t\tAND p.domain_id = $3\n\t\tAND p.path = ANY($4)\n\t\tAND t.priority_score < $1",
    "calls": 122153,
    "total_time": 278067.413357001,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 151430,
    "total_time": 228553.086374001,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "UPDATE jobs \n\t\t\t\tSET status = $1, completed_at = NOW()\n\t\t\t\tWHERE (completed_tasks + failed_tasks) >= (total_tasks - COALESCE(skipped_tasks, $2))\n\t\t\t\t  AND status = $3\n\t\t\t\tRETURNING id",
    "calls": 911402,
    "total_time": 182117.871199,
    "prop_total_time": "0.1%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score \n\t\t\tFROM tasks \n\t\t\tWHERE status = $2\n\t\t AND job_id = $1\n\t\t\tORDER BY priority_score DESC, created_at ASC\n\t\t\tLIMIT $3\n\t\t\tFOR UPDATE SKIP LOCKED",
    "calls": 194342,
    "total_time": 174980.741236,
    "prop_total_time": "0.1%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 15.44,
      "startup_cost_after": 15.44,
      "total_cost_before": 38.84,
      "total_cost_after": 38.84,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO tasks (\n\t\t\t\tid, job_id, page_id, path, status, created_at, retry_count,\n\t\t\t\tsource_type, source_url\n\t\t\t) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
    "calls": 120492,
    "total_time": 113661.429161,
    "prop_total_time": "0.0%",
    "index_advisor_result": null
  },
  {
    "rolname": "authenticator",
    "query": "SELECT name FROM pg_timezone_names",
    "calls": 913,
    "total_time": 112445.245508,
    "prop_total_time": "0.0%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": 0,
      "startup_cost_after": 0,
      "total_cost_before": 10,
      "total_cost_after": 10,
      "index_statements": []
    }
  },
  {
    "rolname": "postgres",
    "query": "INSERT INTO pages (domain_id, path)\n\t\t\tVALUES ($1, $2)\n\t\t\tON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path\n\t\t\tRETURNING id",
    "calls": 36658,
    "total_time": 111746.023833,
    "prop_total_time": "0.0%",
    "index_advisor_result": null
  },
  {
    "rolname": "postgres",
    "query": "SELECT id, job_id, url, depth, created_at, retry_count, source_type, source_url \n\t\t\tFROM tasks \n\t\t\tWHERE status = $1\n\t\t\n\t\t\tORDER BY created_at ASC\n\t\t\tLIMIT $2\n\t\t\tFOR UPDATE SKIP LOCKED",
    "calls": 6198401,
    "total_time": 92599.4945479757,
    "prop_total_time": "0.0%",
    "index_advisor_result": {
      "has_suggestion": null,
      "startup_cost_before": null,
      "startup_cost_after": null,
      "total_cost_before": null,
      "total_cost_after": null,
      "index_statements": []
    }
  }
]rolname,query,calls,total_time,prop_total_time,index_advisor_result
postgres,"UPDATE tasks
			SET status = $3, started_at = $1
			WHERE id = $2",266565,77102527.6181111,31.9%,null
postgres,"UPDATE tasks 
				SET status = $1, completed_at = $2, status_code = $3, 
					response_time = $4, cache_status = $5, content_type = $6,
					content_length = $7, headers = $8, redirect_url = $9,
					dns_lookup_time = $10, tcp_connection_time = $11, tls_handshake_time = $12,
					ttfb = $13, content_transfer_time = $14,
					second_response_time = $15, second_cache_status = $16,
					second_content_length = $17, second_headers = $18,
					second_dns_lookup_time = $19, second_tcp_connection_time = $20,
					second_tls_handshake_time = $21, second_ttfb = $22,
					second_content_transfer_time = $23,
					retry_count = $24, cache_check_attempts = $25
				WHERE id = $26",37207,63722633.6756994,26.4%,null
postgres,"UPDATE jobs
			SET total_tasks = total_tasks + $1,
				skipped_tasks = skipped_tasks + $2
			WHERE id = $3",90382,51624756.1642233,21.4%,null
postgres,"INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id",11440436,36404808.8562005,15.1%,null
postgres,"UPDATE tasks 
				SET status = $1
				WHERE id = $2",1638,5027459.382934,2.1%,null
postgres,"UPDATE tasks 
				SET status = $1, completed_at = $2, status_code = $3, 
					response_time = $4, cache_status = $5, content_type = $6,
					second_response_time = $7, second_cache_status = $8,
					retry_count = $9, cache_check_attempts = $10
				WHERE id = $11",13213,1953713.607124,0.8%,null
postgres,"INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id",689109,970622.612879021,0.4%,null
postgres,"INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at, retry_count,
				source_type, source_url, priority_score
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",205965,709190.480755987,0.3%,null
postgres,"SELECT t.id, t.job_id, t.page_id, p.path, t.retry_count 
			FROM tasks t
			JOIN pages p ON t.page_id = p.id
			WHERE status = $1 
			AND started_at < $2",76706,629021.952581001,0.3%,"{""has_suggestion"":true,""startup_cost_before"":2540.54,""startup_cost_after"":2540.59,""total_cost_before"":27483.43,""total_cost_after"":12766.08,""index_statements"":[""CREATE INDEX ON public.tasks USING btree (started_at)""]}"
postgres,"UPDATE tasks 
				SET status = $1, completed_at = $2, error = $3, retry_count = $4
				WHERE id = $5",4732,598439.623715,0.2%,null
postgres,"UPDATE tasks 
				SET status = $1, completed_at = $2, status_code = $3, 
					response_time = $4, cache_status = $5, content_type = $6,
					second_response_time = $7, second_cache_status = $8
				WHERE id = $9",175793,547366.066530999,0.2%,null
postgres,"SELECT max_pages, 
				   COALESCE((SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status != $2), $3)
			FROM jobs WHERE id = $1",90654,333628.688332001,0.1%,"{""has_suggestion"":null,""startup_cost_before"":528.77,""startup_cost_after"":528.77,""total_cost_before"":530.99,""total_cost_after"":530.99,""index_statements"":[]}"
postgres,"UPDATE tasks t
		SET priority_score = $1
		FROM pages p
		WHERE t.page_id = p.id
		AND t.job_id = $2
		AND p.domain_id = $3
		AND p.path = ANY($4)
		AND t.priority_score < $1",122153,278067.413357001,0.1%,null
postgres,"INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id",151430,228553.086374001,0.1%,null
postgres,"UPDATE jobs 
				SET status = $1, completed_at = NOW()
				WHERE (completed_tasks + failed_tasks) >= (total_tasks - COALESCE(skipped_tasks, $2))
				  AND status = $3
				RETURNING id",911402,182117.871199,0.1%,null
postgres,"SELECT id, job_id, page_id, path, created_at, retry_count, source_type, source_url, priority_score 
			FROM tasks 
			WHERE status = $2
		 AND job_id = $1
			ORDER BY priority_score DESC, created_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED",194342,174980.741236,0.1%,"{""has_suggestion"":null,""startup_cost_before"":15.44,""startup_cost_after"":15.44,""total_cost_before"":38.84,""total_cost_after"":38.84,""index_statements"":[]}"
postgres,"INSERT INTO tasks (
				id, job_id, page_id, path, status, created_at, retry_count,
				source_type, source_url
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",120492,113661.429161,0.0%,null
authenticator,SELECT name FROM pg_timezone_names,913,112445.245508,0.0%,"{""has_suggestion"":null,""startup_cost_before"":0,""startup_cost_after"":0,""total_cost_before"":10,""total_cost_after"":10,""index_statements"":[]}"
postgres,"INSERT INTO pages (domain_id, path)
			VALUES ($1, $2)
			ON CONFLICT (domain_id, path) DO UPDATE SET path = EXCLUDED.path
			RETURNING id",36658,111746.023833,0.0%,null
postgres,"SELECT id, job_id, url, depth, created_at, retry_count, source_type, source_url 
			FROM tasks 
			WHERE status = $1
		
			ORDER BY created_at ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED",6198401,92599.4945479757,0.0%,"{""has_suggestion"":null,""startup_cost_before"":null,""startup_cost_after"":null,""total_cost_before"":null,""total_cost_after"":null,""index_statements"":[]}"