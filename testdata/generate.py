#!/usr/bin/env python3
"""Generate small parquet test fixtures for ATB CLI tests.

Each fixture has 20 rows with realistic data matching production schemas.
Run this script to regenerate fixtures after schema changes.
"""

import pyarrow as pa
import pyarrow.parquet as pq
from pathlib import Path

OUTPUT_DIR = Path(__file__).parent / "fixtures"
OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

SAMPLE_IDS = [f"SAMN{i:08d}" for i in range(1, 21)]
RUN_IDS = [f"SRR{i:04d}" for i in range(1, 21)]

# Species distribution: E.coli x5, S.aureus x3, S.enterica x3, K.pneumoniae x2,
# P.aeruginosa x2, S.pyogenes x2, M.tuberculosis x2, unknown x1
SPECIES = [
    "Escherichia coli",       # SAMN1
    "Escherichia coli",       # SAMN2
    "Staphylococcus aureus",  # SAMN3
    "Staphylococcus aureus",  # SAMN4
    "Salmonella enterica",    # SAMN5
    "Salmonella enterica",    # SAMN6
    "Klebsiella pneumoniae",  # SAMN7
    "Staphylococcus aureus",  # SAMN8
    "Klebsiella pneumoniae",  # SAMN9
    "Pseudomonas aeruginosa", # SAMN10
    "Escherichia coli",       # SAMN11
    "Escherichia coli",       # SAMN12
    "Pseudomonas aeruginosa", # SAMN13
    "Streptococcus pyogenes", # SAMN14
    "Streptococcus pyogenes", # SAMN15
    "Mycobacterium tuberculosis", # SAMN16
    "Mycobacterium tuberculosis", # SAMN17
    "Salmonella enterica",    # SAMN18
    "Escherichia coli",       # SAMN19
    "unknown",                # SAMN20
]

# hq_filter: 17 PASS, 3 failures at SAMN4, SAMN8, SAMN17
HQ_FILTER = [
    "PASS", "PASS", "PASS", "SYLPH_RESULTS_FAIL",
    "PASS", "PASS", "PASS", "CONTAMINATION_FAIL",
    "PASS", "PASS", "PASS", "PASS",
    "PASS", "PASS", "PASS", "PASS",
    "SYLPH_RESULTS_FAIL", "PASS", "PASS", "PASS",
]

# dataset: 661k for SAMN1-4, SAMN15-17 (7 total), rest mixed
DATASETS = [
    "661k", "661k", "661k", "661k",              # SAMN1-4
    "r0.2", "r0.2", "r0.2", "r0.2",              # SAMN5-8
    "Incr_release.202408", "Incr_release.202408", # SAMN9-10
    "Incr_release.202408", "Incr_release.202408", # SAMN11-12
    "Incr_release.202505", "Incr_release.202505", # SAMN13-14
    "661k", "661k", "661k",                       # SAMN15-17
    "Incr_release.202505", "r0.2",                # SAMN18-19
    "Incr_release.202505",                        # SAMN20
]

ASM_FASTA_ON_OSF = [1] * 20
ASM_FASTA_ON_OSF[11] = 0  # SAMN12 = 0

ASSEMBLY_ACCESSIONS = [
    "GCA_000001405.15", "GCA_000001635.9", "GCA_000001635.8", "NA",
    "GCA_000006945.2", "GCA_000006925.2", "GCA_000240185.2", "NA",
    "GCA_000240185.1", "GCA_000006765.1", "GCA_000001405.28", "GCA_000001635.7",
    "GCA_000006765.2", "GCA_900475405.1", "GCA_900475405.2", "GCA_000195955.2",
    "NA", "GCA_000006945.1", "GCA_000001405.14", "NA",
]

AWS_URL_TEMPLATE = "s3://atb-assemblies/{sample}/{sample}_assembly.fasta.gz"
OSF_TARBALL_TEMPLATE = "https://osf.io/download/{code}/"
OSF_FILENAME_TEMPLATE = "{sample}_assembly.tar.gz"
OSF_CODES = [f"ab{i:03d}xy" for i in range(1, 21)]


def ls(s: str) -> pa.Array:
    """Shorthand: cast a list of strings to large_string array."""
    return pa.array(s, type=pa.large_string())


def generate_assembly():
    n = 20
    table = pa.table(
        {
            "sample_accession": ls(SAMPLE_IDS),
            "run_accession": ls(RUN_IDS),
            "assembly_accession": ls(ASSEMBLY_ACCESSIONS),
            "sylph_species": ls(SPECIES),
            "hq_filter": ls(HQ_FILTER),
            "asm_fasta_on_osf": pa.array(ASM_FASTA_ON_OSF, type=pa.int64()),
            "dataset": ls(DATASETS),
            "scientific_name": ls(SPECIES),
            "aws_url": ls([AWS_URL_TEMPLATE.format(sample=s) for s in SAMPLE_IDS]),
            "osf_tarball_url": ls([OSF_TARBALL_TEMPLATE.format(code=c) for c in OSF_CODES]),
            "osf_tarball_filename": ls([OSF_FILENAME_TEMPLATE.format(sample=s) for s in SAMPLE_IDS]),
            "assembly_seqkit_sum": ls([""] * n),
            "asm_pipe_filter": ls([""] * n),
            "sylph_species_pre_202505": ls([""] * n),
            "in_hq_pre_202505": ls([""] * n),
            "sylph_filter": ls([""] * n),
            "comments": ls([""] * n),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "assembly.parquet")
    print(f"assembly.parquet: {len(table)} rows")


def generate_assembly_stats():
    # E. coli N50 values: SAMN1=234000, SAMN2=245000, SAMN11=250000, SAMN12=230000, SAMN19=240000
    n50_map = {0: 234000, 1: 245000, 10: 250000, 11: 230000, 18: 240000}
    n50_defaults = {
        "Staphylococcus aureus": 180000,
        "Salmonella enterica": 220000,
        "Klebsiella pneumoniae": 260000,
        "Pseudomonas aeruginosa": 300000,
        "Streptococcus pyogenes": 150000,
        "Mycobacterium tuberculosis": 350000,
        "unknown": 100000,
    }

    n50_vals = []
    for i, sp in enumerate(SPECIES):
        if i in n50_map:
            n50_vals.append(n50_map[i])
        else:
            n50_vals.append(n50_defaults.get(sp, 200000))

    genome_size_approx = {
        "Escherichia coli": 5000000,
        "Staphylococcus aureus": 2800000,
        "Salmonella enterica": 4800000,
        "Klebsiella pneumoniae": 5700000,
        "Pseudomonas aeruginosa": 6300000,
        "Streptococcus pyogenes": 1900000,
        "Mycobacterium tuberculosis": 4400000,
        "unknown": 3000000,
    }

    total_lengths = [genome_size_approx.get(sp, 3000000) for sp in SPECIES]
    numbers = [total // (n50 // 2) for total, n50 in zip(total_lengths, n50_vals)]

    table = pa.table(
        {
            "sample_accession": pa.array(SAMPLE_IDS, type=pa.large_string()),
            "total_length": pa.array(total_lengths, type=pa.int64()),
            "number": pa.array(numbers, type=pa.int64()),
            "mean_length": pa.array([tl // n for tl, n in zip(total_lengths, numbers)], type=pa.int64()),
            "longest": pa.array([n50 * 3 for n50 in n50_vals], type=pa.int64()),
            "shortest": pa.array([500] * 20, type=pa.int64()),
            "N_count": pa.array([0] * 20, type=pa.int64()),
            "Gaps": pa.array([0] * 20, type=pa.int64()),
            "N50": pa.array(n50_vals, type=pa.int64()),
            "N50n": pa.array([n // 4 for n in numbers], type=pa.int64()),
            "N70": pa.array([n50 * 7 // 10 for n50 in n50_vals], type=pa.int64()),
            "N70n": pa.array([n // 3 for n in numbers], type=pa.int64()),
            "N90": pa.array([n50 // 2 for n50 in n50_vals], type=pa.int64()),
            "N90n": pa.array([n // 2 for n in numbers], type=pa.int64()),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "assembly_stats.parquet")
    print(f"assembly_stats.parquet: {len(table)} rows")


def generate_checkm2():
    # E. coli completeness: SAMN1=99.5, SAMN2=99.2, SAMN11=99.3, SAMN12=97.0, SAMN19=99.4
    completeness_map = {0: 99.5, 1: 99.2, 10: 99.3, 11: 97.0, 18: 99.4}
    species_completeness = {
        "Staphylococcus aureus": [98.5, 97.8, 98.1],
        "Salmonella enterica": [98.9, 99.0, 98.7],
        "Klebsiella pneumoniae": [97.5, 96.8],
        "Pseudomonas aeruginosa": [96.5, 97.2],
        "Streptococcus pyogenes": [98.0, 97.5],
        "Mycobacterium tuberculosis": [99.1, 99.0],
        "unknown": [75.0],
    }
    species_counters = {k: 0 for k in species_completeness}

    completeness_vals = []
    contamination_vals = []
    for i, sp in enumerate(SPECIES):
        if i in completeness_map:
            completeness_vals.append(completeness_map[i])
        else:
            vals = species_completeness.get(sp, [90.0])
            idx = species_counters.get(sp, 0) % len(vals)
            completeness_vals.append(vals[idx])
            species_counters[sp] = idx + 1
        # contamination: failed samples get higher values
        if HQ_FILTER[i] == "CONTAMINATION_FAIL":
            contamination_vals.append(15.0)
        elif HQ_FILTER[i] != "PASS":
            contamination_vals.append(8.5)
        else:
            contamination_vals.append(round(0.2 + (i % 10) * 0.3, 1))

    table = pa.table(
        {
            "sample_accession": pa.array(SAMPLE_IDS, type=pa.large_string()),
            "Completeness_General": pa.array(completeness_vals, type=pa.float64()),
            "Contamination": pa.array(contamination_vals, type=pa.float64()),
            "Completeness_Specific": pa.array(completeness_vals, type=pa.float64()),
            "Completeness_Model_Used": pa.array(
                ["Neural_Network_General_Model"] * 20, type=pa.large_string()
            ),
            "Translation_Table_Used": pa.array([11] * 20, type=pa.int64()),
            "Coding_Density": pa.array([round(0.85 + (i % 5) * 0.01, 3) for i in range(20)], type=pa.float64()),
            "Contig_N50": pa.array([50000 + i * 5000 for i in range(20)], type=pa.int64()),
            "Average_Gene_Length": pa.array([900 + i * 5 for i in range(20)], type=pa.int64()),
            "Genome_Size": pa.array([3000000 + i * 200000 for i in range(20)], type=pa.int64()),
            "GC_Content": pa.array([round(0.40 + (i % 15) * 0.01, 3) for i in range(20)], type=pa.float64()),
            "Total_Coding_Sequences": pa.array([2800 + i * 50 for i in range(20)], type=pa.int64()),
            "Additional_Notes": pa.array([""] * 20, type=pa.large_string()),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "checkm2.parquet")
    print(f"checkm2.parquet: {len(table)} rows")


def generate_run():
    # in_661k: match dataset = 661k (SAMN1-4, SAMN15-17)
    in_661k = [1 if d == "661k" else 0 for d in DATASETS]

    table = pa.table(
        {
            "run_accession": pa.array(RUN_IDS, type=pa.large_string()),
            "sample_accession": pa.array(SAMPLE_IDS, type=pa.large_string()),
            "in_661k": pa.array(in_661k, type=pa.int64()),
            "in_ena_20240625": pa.array([1] * 10 + [0] * 10, type=pa.int64()),
            "in_ena_20240801": pa.array([1] * 14 + [0] * 6, type=pa.int64()),
            "in_ena_20250506": pa.array([1] * 20, type=pa.int64()),
            "ena_202505_batch": pa.array(
                [f"batch_{(i // 5) + 1}" for i in range(20)], type=pa.large_string()
            ),
            "fastq_md5": pa.array(
                [f"{'a' * 32}" if i % 3 != 0 else "" for i in range(20)],
                type=pa.large_string(),
            ),
            "meta_pass_atb": pa.array([1 if hq == "PASS" else 0 for hq in HQ_FILTER], type=pa.int64()),
            "meta_pass_661k": pa.array(in_661k, type=pa.int64()),
            "pass": pa.array([1 if hq == "PASS" else 0 for hq in HQ_FILTER], type=pa.int64()),
            "comments": pa.array([""] * 20, type=pa.large_string()),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "run.parquet")
    print(f"run.parquet: {len(table)} rows")


def generate_ena():
    countries = [
        "UK", "US", "Germany", "Australia", "France",
        "Japan", "China", "India", "South Africa", "Brazil",
        "UK", "US", "Germany", "Australia", "France",
        "Japan", "China", "India", "South Africa", "Brazil",
    ]
    platforms = ["ILLUMINA"] * 16 + ["OXFORD_NANOPORE"] * 4
    models = (
        ["Illumina NovaSeq 6000"] * 10
        + ["Illumina HiSeq 2500"] * 6
        + ["Oxford Nanopore MinION"] * 4
    )
    library_strategy = ["WGS"] * 18 + ["AMPLICON", "WGS"]
    library_source = ["GENOMIC"] * 20
    library_layout = ["PAIRED"] * 17 + ["SINGLE"] * 3

    collection_dates = [
        "2019-01-15", "2019-03-22", "2019-06-10", "2019-08-05",
        "2020-01-30", "2020-04-12", "2020-07-18", "2020-10-25",
        "2021-02-08", "2021-05-14", "2021-08-22", "2021-11-30",
        "2022-01-07", "2022-04-19", "2022-06-28", "2022-09-15",
        "2023-01-03", "2023-05-20", "2023-08-11", "2023-12-01",
    ]

    fastq_ftp = [
        f"ftp.sra.ebi.ac.uk/vol1/fastq/{r[:6]}/{r}/{r}_1.fastq.gz"
        for r in RUN_IDS
    ]
    fastq_bytes = [500_000_000 + i * 50_000_000 for i in range(20)]
    read_counts = [5_000_000 + i * 200_000 for i in range(20)]
    base_counts = [rc * 150 for rc in read_counts]

    study_ids = [f"SRP{i:06d}" for i in range(1, 21)]
    experiment_ids = [f"SRX{i:06d}" for i in range(1, 21)]

    table = pa.table(
        {
            "run_accession": pa.array(RUN_IDS, type=pa.large_string()),
            "sample_accession": pa.array(SAMPLE_IDS, type=pa.large_string()),
            "country": pa.array(countries, type=pa.large_string()),
            "collection_date": pa.array(collection_dates, type=pa.large_string()),
            "instrument_platform": pa.array(platforms, type=pa.large_string()),
            "instrument_model": pa.array(models, type=pa.large_string()),
            "scientific_name": pa.array(SPECIES, type=pa.large_string()),
            "fastq_ftp": pa.array(fastq_ftp, type=pa.large_string()),
            "fastq_bytes": pa.array(fastq_bytes, type=pa.int64()),
            "read_count": pa.array(read_counts, type=pa.int64()),
            "base_count": pa.array(base_counts, type=pa.int64()),
            "library_strategy": pa.array(library_strategy, type=pa.large_string()),
            "library_source": pa.array(library_source, type=pa.large_string()),
            "library_layout": pa.array(library_layout, type=pa.large_string()),
            "study_accession": pa.array(study_ids, type=pa.large_string()),
            "experiment_accession": pa.array(experiment_ids, type=pa.large_string()),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "ena_20250506.parquet")
    print(f"ena_20250506.parquet: {len(table)} rows")


def generate_mlst():
    # MLST scheme mapping by species
    # E. coli: ecoli_achtman_4 (SAMN1, SAMN2, SAMN11, SAMN12, SAMN19)
    # S. aureus: saureus (SAMN3, SAMN4, SAMN8)
    # S. enterica: salmonella (SAMN5, SAMN6, SAMN18)
    # K. pneumoniae: klebsiella (SAMN7, SAMN9)
    # P. aeruginosa: paeruginosa (SAMN10, SAMN13)
    # S. pyogenes: spyogenes (SAMN14, SAMN15)
    # M. tuberculosis: "-" (not in MLST database)
    # unknown: "-"
    scheme_map = {
        "Escherichia coli": "ecoli_achtman_4",
        "Staphylococcus aureus": "saureus",
        "Salmonella enterica": "salmonella",
        "Klebsiella pneumoniae": "klebsiella",
        "Pseudomonas aeruginosa": "paeruginosa",
        "Streptococcus pyogenes": "spyogenes",
        "Mycobacterium tuberculosis": "-",
        "unknown": "-",
    }
    # ST numbers per species (cycling through a few)
    st_map = {
        "Escherichia coli": ["131", "131", "73", "10", "131"],
        "Staphylococcus aureus": ["8", "22", "8"],
        "Salmonella enterica": ["11", "19", "34"],
        "Klebsiella pneumoniae": ["258", "45"],
        "Pseudomonas aeruginosa": ["175", "235"],
        "Streptococcus pyogenes": ["28", "36"],
        "Mycobacterium tuberculosis": ["-", "-"],
        "unknown": ["-"],
    }
    status_map = {
        "Escherichia coli": ["PERFECT", "PERFECT", "PERFECT", "OK", "PERFECT"],
        "Staphylococcus aureus": ["PERFECT", "NOVEL", "PERFECT"],
        "Salmonella enterica": ["PERFECT", "PERFECT", "OK"],
        "Klebsiella pneumoniae": ["PERFECT", "PERFECT"],
        "Pseudomonas aeruginosa": ["PERFECT", "OK"],
        "Streptococcus pyogenes": ["PERFECT", "PERFECT"],
        "Mycobacterium tuberculosis": ["NONE", "NONE"],
        "unknown": ["NONE"],
    }
    alleles_map = {
        "Escherichia coli": [
            "adk(10);fumC(11);gyrB(4);icd(8);mdh(8);purA(8);recA(2)",
            "adk(10);fumC(11);gyrB(4);icd(8);mdh(8);purA(8);recA(2)",
            "adk(7);fumC(40);gyrB(47);icd(12);mdh(36);purA(18);recA(15)",
            "adk(3);fumC(6);gyrB(4);icd(5);mdh(15);purA(3);recA(7)",
            "adk(10);fumC(11);gyrB(4);icd(8);mdh(8);purA(8);recA(2)",
        ],
        "Staphylococcus aureus": [
            "arcC(3);aroE(3);glpF(1);gmk(1);pta(4);tpi(4);yqiL(3)",
            "arcC(1);aroE(4);glpF(1);gmk(1);pta(4);tpi(4);yqiL(3)",
            "arcC(3);aroE(3);glpF(1);gmk(1);pta(4);tpi(4);yqiL(3)",
        ],
        "Salmonella enterica": [
            "aroC(10);dnaN(7);hemD(12);hisD(9);purE(7);sucA(3);thrA(3)",
            "aroC(10);dnaN(7);hemD(12);hisD(9);purE(7);sucA(3);thrA(3)",
            "aroC(10);dnaN(7);hemD(12);hisD(9);purE(7);sucA(3);thrA(5)",
        ],
        "Klebsiella pneumoniae": [
            "gapA(3);infB(3);mdh(1);pgi(1);phoE(9);rpoB(4);tonB(12)",
            "gapA(3);infB(3);mdh(1);pgi(1);phoE(9);rpoB(4);tonB(4)",
        ],
        "Pseudomonas aeruginosa": [
            "acsA(28);aroE(5);guaA(4);mutL(4);nuoD(3);ppsA(4);trpE(4)",
            "acsA(28);aroE(5);guaA(4);mutL(4);nuoD(3);ppsA(4);trpE(4)",
        ],
        "Streptococcus pyogenes": [
            "gki(2);gtr(5);murI(4);mutS(2);recP(2);xpt(3);yqiL(3)",
            "gki(1);gtr(5);murI(4);mutS(2);recP(2);xpt(3);yqiL(3)",
        ],
        "Mycobacterium tuberculosis": ["-", "-"],
        "unknown": ["-"],
    }

    species_counters = {sp: 0 for sp in scheme_map}
    schemes, sts, statuses, scores, alleles_list = [], [], [], [], []
    for sp in SPECIES:
        c = species_counters[sp]
        st_list = st_map[sp]
        status_list = status_map[sp]
        alleles_list_sp = alleles_map[sp]
        schemes.append(scheme_map[sp])
        sts.append(st_list[c % len(st_list)])
        statuses.append(status_list[c % len(status_list)])
        # score: PERFECT=100, NOVEL=95, OK=90, NONE=0
        status_val = status_list[c % len(status_list)]
        score_val = {"PERFECT": 100, "NOVEL": 95, "OK": 90, "NONE": 0}.get(status_val, 80)
        scores.append(score_val)
        alleles_list.append(alleles_list_sp[c % len(alleles_list_sp)])
        species_counters[sp] = c + 1

    table = pa.table(
        {
            "sample": pa.array(SAMPLE_IDS, type=pa.large_string()),
            "mlst_scheme": pa.array(schemes, type=pa.large_string()),
            "mlst_st": pa.array(sts, type=pa.large_string()),
            "mlst_status": pa.array(statuses, type=pa.large_string()),
            "mlst_score": pa.array(scores, type=pa.int32()),
            "mlst_alleles": pa.array(alleles_list, type=pa.large_string()),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "mlst.parquet")
    print(f"mlst.parquet: {len(table)} rows")


def generate_amrfinderplus():
    """Write a 15-row AMR fixture mirroring the AMRFinderPlus v4.2.5 schema.

    The parquet file contains all 26 columns AMRFinderPlus produces, with the
    column names byte-for-byte identical to the production data on OSF.

    Test expectations encoded here (see internal/amr/amr_test.go):
      - 15 rows total: 10 AMR + 5 STRESS
      - 9 rows with genus="Escherichia"
      - 4 bla* genes (blaTEM-1, blaEC, blaCTX-M-15, blaZ)
      - At least one row with class="EFFLUX"
      - Coverage values both above and below 99.0
    """
    # Each row tuple: (name, gene, type, subtype, cov, ident, method, class,
    #                  subclass, species, genus)
    base_rows = [
        # 10 AMR rows
        ("SAMN00000001", "blaTEM-1",   "AMR", "AMR", 100.0, 100.00, "ALLELEX", "BETA-LACTAM",   "BETA-LACTAM",   "Escherichia coli",        "Escherichia"),
        ("SAMN00000001", "acrF",       "AMR", "AMR", 100.0,  98.07, "BLASTX",  "EFFLUX",        "EFFLUX",        "Escherichia coli",        "Escherichia"),
        ("SAMN00000001", "blaEC",      "AMR", "AMR", 100.0,  92.57, "BLASTX",  "BETA-LACTAM",   "BETA-LACTAM",   "Escherichia coli",        "Escherichia"),
        ("SAMN00000002", "blaTEM-1",   "AMR", "AMR", 100.0, 100.00, "ALLELEX", "BETA-LACTAM",   "BETA-LACTAM",   "Escherichia coli",        "Escherichia"),
        ("SAMN00000002", "sul1",       "AMR", "AMR", 100.0, 100.00, "EXACTX",  "SULFONAMIDE",   "SULFONAMIDE",   "Escherichia coli",        "Escherichia"),
        ("SAMN00000003", "mecA",       "AMR", "AMR", 100.0,  99.50, "EXACTX",  "METHICILLIN",   "METHICILLIN",   "Staphylococcus aureus",   "Staphylococcus"),
        ("SAMN00000003", "blaZ",       "AMR", "AMR", 100.0,  98.00, "BLASTX",  "BETA-LACTAM",   "BETA-LACTAM",   "Staphylococcus aureus",   "Staphylococcus"),
        ("SAMN00000005", "blaCTX-M-15","AMR", "AMR", 100.0, 100.00, "ALLELEX", "BETA-LACTAM",   "BETA-LACTAM",   "Salmonella enterica",     "Salmonella"),
        ("SAMN00000005", "aadA1",      "AMR", "AMR", 100.0, 100.00, "EXACTX",  "AMINOGLYCOSIDE","STREPTOMYCIN",  "Salmonella enterica",     "Salmonella"),
        ("SAMN00000005", "tet(A)",     "AMR", "AMR", 100.0, 100.00, "EXACTX",  "TETRACYCLINE",  "TETRACYCLINE",  "Salmonella enterica",     "Salmonella"),
        # 5 STRESS rows
        ("SAMN00000001", "emrE",       "STRESS", "BIOCIDE", 100.0, 96.36, "BLASTX", "EFFLUX", "EFFLUX", "Escherichia coli",      "Escherichia"),
        ("SAMN00000001", "fieF",       "STRESS", "METAL",   100.0, 97.67, "BLASTX", "",       "",       "Escherichia coli",      "Escherichia"),
        ("SAMN00000002", "emrE",       "STRESS", "BIOCIDE", 100.0, 96.36, "BLASTX", "EFFLUX", "EFFLUX", "Escherichia coli",      "Escherichia"),
        ("SAMN00000002", "fieF",       "STRESS", "METAL",    95.0, 97.67, "BLASTX", "",       "",       "Escherichia coli",      "Escherichia"),
        ("SAMN00000003", "emrE",       "STRESS", "BIOCIDE",  85.0, 96.36, "BLASTX", "EFFLUX", "EFFLUX", "Staphylococcus aureus", "Staphylococcus"),
    ]

    n = len(base_rows)
    names               = [r[0] for r in base_rows]
    gene_symbols        = [r[1] for r in base_rows]
    types               = [r[2] for r in base_rows]
    subtypes            = [r[3] for r in base_rows]
    coverages           = [r[4] for r in base_rows]
    identities          = [r[5] for r in base_rows]
    methods             = [r[6] for r in base_rows]
    classes             = [r[7] for r in base_rows]
    subclasses          = [r[8] for r in base_rows]
    species             = [r[9] for r in base_rows]
    genera              = [r[10] for r in base_rows]

    # Synthesise the remaining 15 columns deterministically so callers can
    # verify they round-trip through the schema. Realistic-ish values:
    #   - Protein id / Contig id: NCBI-style accessions
    #   - Start / Stop / Strand: contiguous coordinates on alternating strands
    #   - Element name: human-readable copy of the gene
    #   - Scope: the AMRFP "core" scope is used for everything we ship
    #   - Target / reference / alignment lengths: monotonically increasing
    #   - Closest reference accession + name: AMRFP refgene-style placeholders
    #   - HMM accession + description: NF/HMM placeholders
    #   - Hierarchy node: lowercased gene symbol (matches AMRFP convention)
    protein_ids = [f"WP_{1000000 + i:09d}.1" for i in range(n)]
    contig_ids = [f"contig_{i + 1:03d}" for i in range(n)]
    starts = [1000 + i * 1000 for i in range(n)]
    stops = [s + 800 for s in starts]
    strands = ["+" if i % 2 == 0 else "-" for i in range(n)]
    element_names = [f"{gene} resistance protein" for gene in gene_symbols]
    scopes = ["core"] * n
    target_lengths = [800 + i * 5 for i in range(n)]
    ref_seq_lengths = [810 + i * 5 for i in range(n)]
    alignment_lengths = [780 + i * 5 for i in range(n)]
    closest_accessions = [f"REF_{i + 1:04d}" for i in range(n)]
    closest_names = [f"{gene} reference" for gene in gene_symbols]
    hmm_accessions = [f"NF{i + 1:06d}.1" if i % 2 == 0 else "" for i in range(n)]
    hmm_descriptions = [
        f"{gene} HMM" if acc else ""
        for gene, acc in zip(gene_symbols, hmm_accessions)
    ]
    hierarchy_nodes = [gene.lower() for gene in gene_symbols]

    table = pa.table(
        {
            "Name":                        ls(names),
            "Protein id":                  ls(protein_ids),
            "Contig id":                   ls(contig_ids),
            "Start":                       pa.array(starts, type=pa.int64()),
            "Stop":                        pa.array(stops, type=pa.int64()),
            "Strand":                      ls(strands),
            "Element symbol":              ls(gene_symbols),
            "Element name":                ls(element_names),
            "Scope":                       ls(scopes),
            "Type":                        ls(types),
            "Subtype":                     ls(subtypes),
            "Class":                       ls(classes),
            "Subclass":                    ls(subclasses),
            "Method":                      ls(methods),
            "Target length":               pa.array(target_lengths, type=pa.int64()),
            "Reference sequence length":   pa.array(ref_seq_lengths, type=pa.int64()),
            "% Coverage of reference":     pa.array(coverages, type=pa.float64()),
            "% Identity to reference":     pa.array(identities, type=pa.float64()),
            "Alignment length":            pa.array(alignment_lengths, type=pa.int64()),
            "Closest reference accession": ls(closest_accessions),
            "Closest reference name":      ls(closest_names),
            "HMM accession":               ls(hmm_accessions),
            "HMM description":             ls(hmm_descriptions),
            "Hierarchy node":              ls(hierarchy_nodes),
            "genus":                       ls(genera),
            "species":                     ls(species),
        }
    )
    pq.write_table(table, OUTPUT_DIR / "amrfinderplus.parquet")
    print(f"amrfinderplus.parquet: {len(table)} rows")


if __name__ == "__main__":
    generate_assembly()
    generate_assembly_stats()
    generate_checkm2()
    generate_run()
    generate_ena()
    generate_mlst()
    generate_amrfinderplus()
    print(f"\nAll fixtures written to {OUTPUT_DIR}")
