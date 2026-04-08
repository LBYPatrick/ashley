# Benchmark Results

Every optimization MUST include before/after benchmark evidence with rigorous statistical analysis. Do not rely on theoretical analysis alone — measure.

## Procedure

1. **Before optimizing:** write or identify a benchmark that exercises the hot path. Use the language's standard benchmarking tools:
   - Python: `timeit`, `time.perf_counter`, or `pytest-benchmark`
   - TypeScript/JavaScript: `performance.now()`, `console.time`, or `vitest bench`
   - Rust: `criterion` or `#[bench]`
   - Go: `testing.B` benchmarks
   - General: wall-clock timing with multiple iterations
2. **Determine trial count dynamically.** The total benchmark time for each phase (before and after) MUST NOT exceed **5 minutes**:
   - Run **1 warmup trial** first to estimate per-trial duration.
   - Compute max trials that fit in 5 minutes: `max_trials = floor(300s / trial_duration)`.
   - Use `N = clamp(max_trials, 3, 100)` — at least 3 trials (minimum for any statistics), at most 100 (diminishing returns).
   - If a single trial takes > 100 seconds, use exactly 3 trials.
   - Use the **same N** for both before and after runs so degrees of freedom are comparable.
3. **Run the baseline** benchmark for N trials. Record every individual measurement.
4. **After optimizing:** re-run the same benchmark under identical conditions with the same N trials.
5. **Compute statistics** for both before and after:
   - **Mean** (average)
   - **Min** and **Max**
   - **Standard deviation** (stdev)
   - **Median** (p50)
   - **p95** and **p99** (if N >= 20; omit for smaller sample sizes)
6. **Statistical significance test:** run Welch's t-test with **alpha = 0.05**. Report:
   - t-statistic
   - p-value
   - Whether p < 0.05 (significant) or not
   - 95% confidence interval for the difference in means
   - **Degrees of freedom** (Welch-Satterthwaite approximation)
   - **Confidence qualifier** based on degrees of freedom:
     - df >= 29: "high confidence" (equivalent to 30+ trials per group)
     - 10 <= df < 29: "moderate confidence" — results are directionally reliable but could shift with more data
     - df < 10: "low confidence" — treat as indicative only; flag to reviewer that sample size was limited by trial duration
7. **Report results** in a clear table:
   ```
   Benchmark: <name> | Trials: 30 | Per-trial: ~4.2s | Total time: ~4m12s

   | Metric     | Before (baseline) | After (optimized) |
   |------------|-------------------|-------------------|
   | Mean       | 120.3 ms          | 45.1 ms           |
   | Median     | 118.7 ms          | 44.8 ms           |
   | Min        | 110.2 ms          | 42.1 ms           |
   | Max        | 145.6 ms          | 52.3 ms           |
   | Stdev      | 8.4 ms            | 2.9 ms            |
   | p95        | 135.2 ms          | 49.7 ms           |
   | p99        | 142.1 ms          | 51.8 ms           |

   Welch's t-test: t=45.23, df=38.7, p=1.2e-42 (p < 0.05 ✓ significant)
   95% CI for improvement: [72.8 ms, 77.6 ms]
   Speedup: 2.67x | Confidence: high (df >= 29)
   ```

   For low-N benchmarks (slow trials):
   ```
   Benchmark: <name> | Trials: 3 | Per-trial: ~105s | Total time: ~5m15s

   | Metric     | Before (baseline) | After (optimized) |
   |------------|-------------------|-------------------|
   | Mean       | 42.1 s            | 18.7 s            |
   | Min        | 40.8 s            | 17.9 s            |
   | Max        | 43.9 s            | 19.8 s            |
   | Stdev      | 1.6 s             | 1.0 s             |

   Welch's t-test: t=21.4, df=3.2, p=0.0002 (p < 0.05 ✓ significant)
   95% CI for improvement: [20.9 s, 25.9 s]
   Speedup: 2.25x | Confidence: low (df < 10) ⚠️ limited by trial duration
   ```
8. **Include in commit/PR:** paste the benchmark table and statistical test results into the commit message body and PR description so reviewers can verify the improvement.

## Rules

- If the benchmark shows no **statistically significant** improvement (p >= 0.05), reconsider whether the optimization is worth the added complexity.
- Even if p < 0.05, if the practical improvement is negligible (< 5% mean improvement), reconsider the trade-off.
- When confidence is "low" or "moderate", note this prominently so reviewers know the evidence is weaker. Consider whether the optimization can be benchmarked with a smaller input size to get more trials.
- Always state the benchmark environment (machine, OS, runtime version) so results are reproducible.
- Aim for coefficient of variation (stdev/mean) < 15%. If it's higher and you have room under the 5-minute cap, increase trial count. Otherwise, reduce system noise (close other programs, pin CPU frequency).
- If the optimization trades memory for speed (or vice versa), report both metrics with full statistics.
- For benchmarks in Python, you can use `statistics.mean`, `statistics.stdev`, `scipy.stats.ttest_ind` (with `equal_var=False` for Welch's). For other languages, use equivalent libraries or compute manually.
