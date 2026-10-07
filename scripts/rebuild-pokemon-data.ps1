# Builds a small offline factual dataset, not executable code from another game.
$ErrorActionPreference = 'Stop'
$csvRoot = 'https://raw.githubusercontent.com/PokeAPI/pokeapi/master/data/v2/csv/'
function Read-Data([string]$File) { return ((Invoke-WebRequest ($csvRoot + $File)).Content | ConvertFrom-Csv) }
$species = @(Read-Data 'pokemon_species.csv' | Where-Object { [int]$_.id -le 151 })
$names = @{}; foreach ($row in (Read-Data 'pokemon_species_names.csv')) { if ($row.local_language_id -eq '12' -and [int]$row.pokemon_species_id -le 151) { $names[[int]$row.pokemon_species_id] = $row.name } }
$stats = @{}; foreach ($row in (Read-Data 'pokemon_stats.csv')) { if ([int]$row.pokemon_id -le 151) { $key = [int]$row.pokemon_id; if (-not $stats.ContainsKey($key)) { $stats[$key] = @(0,0,0,0,0,0) }; $stats[$key][[int]$row.stat_id - 1] = [int]$row.base_stat } }
$types = @{}; foreach ($row in (Read-Data 'pokemon_types.csv')) { if ([int]$row.pokemon_id -le 151) { $key = [int]$row.pokemon_id; if (-not $types.ContainsKey($key)) { $types[$key] = @() }; $types[$key] += [int]$row.type_id } }
$experience = @{}; foreach ($row in (Read-Data 'pokemon.csv')) { if ([int]$row.id -le 151) { $experience[[int]$row.id] = [int]$row.base_experience } }
$evolutionRows = @(Read-Data 'pokemon_evolution.csv' | Where-Object { [int]$_.evolved_species_id -le 151 -and $_.is_default -eq '1' })
$parents = @{}; foreach ($row in $species) { if ($row.evolves_from_species_id) { $parents[[int]$row.id] = [int]$row.evolves_from_species_id } }
$itemNames = @{}; foreach ($row in (Read-Data 'items.csv')) { $itemNames[[int]$row.id] = $row.identifier }
$stoneNames = @{ 'fire-stone'='火之石'; 'water-stone'='水之石'; 'thunder-stone'='雷之石'; 'leaf-stone'='叶之石'; 'moon-stone'='月之石' }
$data = @(foreach ($row in $species) {
    $id = [int]$row.id
    $originalTypes = @($types[$id] | ForEach-Object { if ($_ -eq 18) { if ($id -eq 122) { 14 } else { 1 } } elseif ($_ -ne 9) { $_ } } | Select-Object -Unique)
    $evolutions = @(foreach ($entry in $evolutionRows) {
        $target = [int]$entry.evolved_species_id
        if ($parents[$target] -eq $id) {
            $item = ''; if ($entry.trigger_item_id) { $item = $stoneNames[$itemNames[[int]$entry.trigger_item_id]] }
            # Eevee and the stone/trade branches preserve their classic triggers.
            if ([int]$entry.minimum_level -gt 0 -or [int]$entry.evolution_trigger_id -eq 2 -or $item) {
                [pscustomobject][ordered]@{ species=$target; level=[int]$entry.minimum_level; trade=([int]$entry.evolution_trigger_id -eq 2); item=$item }
            }
        }
    })
    $evolutions = @($evolutions | Sort-Object species -Unique)
    [ordered]@{ id=$id; name=$names[$id]; english=$row.identifier; types=$originalTypes; hp=$stats[$id][0]; attack=$stats[$id][1]; defense=$stats[$id][2]; special=$stats[$id][3]; speed=$stats[$id][5]; catchRate=[int]$row.capture_rate; baseExp=$experience[$id]; evolves=$evolutions }
})
$dataDirectory = Join-Path (Split-Path $PSScriptRoot -Parent) 'internal/pokemon'
New-Item -ItemType Directory -Path $dataDirectory -Force | Out-Null
[IO.File]::WriteAllText((Join-Path $dataDirectory 'species.json'), ($data | ConvertTo-Json -Depth 8 -Compress), [Text.UTF8Encoding]::new($false))
$license = (Invoke-WebRequest 'https://raw.githubusercontent.com/PokeAPI/pokeapi/master/LICENSE.md').Content
[IO.File]::WriteAllText((Join-Path $dataDirectory 'POKEAPI_LICENSE.md'), $license, [Text.UTF8Encoding]::new($false))
Write-Host "已生成 $($data.Count) 个第一世代物种的数据；游戏运行无需联网。"
