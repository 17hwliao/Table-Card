"""Rebuild factual Red/Blue species, moves and Red encounters; no ROM/code assets.

Names: PokeAPI CSV. Numeric Gen I facts: pret/pokered data tables.
Downloads are cached outside source in runtime/pokemon-data-cache.
"""
import concurrent.futures, csv, io, json, pathlib, re, urllib.request, time

ROOT = pathlib.Path(__file__).resolve().parents[1]
OUT = ROOT / 'internal/pokemon'
CACHE = ROOT / 'runtime/pokemon-data-cache'
CACHE.mkdir(parents=True, exist_ok=True)
PRET = 'https://raw.githubusercontent.com/pret/pokered/master/'
POKE = 'https://raw.githubusercontent.com/PokeAPI/pokeapi/master/data/v2/csv/'

def fetch(url):
    path = CACHE / re.sub(r'[^a-zA-Z0-9_.-]', '_', url.split('githubusercontent.com/')[-1])
    if path.exists(): return path.read_text(encoding='utf-8')
    for attempt in range(4):
        try:
            with urllib.request.urlopen(url, timeout=45) as response:
                text = response.read().decode('utf-8')
            path.write_text(text, encoding='utf-8')
            return text
        except Exception:
            if attempt == 3: raise
            time.sleep(1 + attempt)

def rows(file): return list(csv.DictReader(io.StringIO(fetch(POKE + file))))
def write(file, value):
    (OUT / file).write_text(json.dumps(value, ensure_ascii=False, separators=(',', ':')), encoding='utf-8')

types = {'NORMAL':1,'FIGHTING':2,'FLYING':3,'POISON':4,'GROUND':5,'ROCK':6,'BUG':7,'GHOST':8,'FIRE':10,'WATER':11,'GRASS':12,'ELECTRIC':13,'PSYCHIC_TYPE':14,'ICE':15,'DRAGON':16}
names = {int(r['move_id']):r['name'] for r in rows('move_names.csv') if r['local_language_id'] == '12'}
rawmoves = []
for line in fetch(PRET+'data/moves/moves.asm').splitlines():
    match = re.match(r'\s*move\s+(\w+),\s*(\w+),\s*(\d+),\s*(\w+),\s*(\d+),\s*(\d+)', line)
    if match:
        key,effect,power,typ,accuracy,pp = match.groups()
        n = len(rawmoves)+1
        rawmoves.append(dict(Key=key,Name=names[n],Effect=effect,Power=int(power),Type=types[typ],Accuracy=int(accuracy),PP=int(pp)))
assert len(rawmoves)==165
# Keep existing saved IDs. 26..176 are deliberately labelled original story skills.
legacy=['TACKLE','THUNDERSHOCK','EMBER','WATER_GUN','VINE_WHIP','CONFUSION','LICK','POISON_STING','ROCK_THROW','EARTHQUAKE','WING_ATTACK','LEECH_LIFE','KARATE_CHOP','ICE_BEAM','DRAGON_RAGE','THUNDER_WAVE','SLEEP_POWDER','HYPNOSIS','POISONPOWDER','SLASH','THUNDERBOLT','FLAMETHROWER','SURF','RAZOR_LEAF','PSYCHIC_M','NIGHT_SHADE']
index={key:i for i,key in enumerate(legacy)}
next_id=177
for move in rawmoves:
    if move['Key'] not in index:
        index[move['Key']]=next_id; next_id+=1
for move in rawmoves: move['ID']=index[move['Key']]
write('moves_gen1.json',rawmoves)

species=json.loads((OUT/'species.json').read_text(encoding='utf-8-sig'))
evos=fetch(PRET+'data/pokemon/evos_moves.asm')
blocks={key.lower():value for key,value in re.findall(r'(?m)^(\w+)EvosMoves:\n(.*?)(?=^\w+EvosMoves:|\Z)',evos,re.S|re.M)}
def stat_species(s):
    filename=s['english'].replace('-','').replace('.','')
    if s['id']==29: filename='nidoranf'
    if s['id']==32: filename='nidoranm'
    if s['id']==83: filename='farfetchd'
    text=fetch(PRET+'data/pokemon/base_stats/'+filename+'.asm')
    vals=re.search(r'db\s+(\d+),\s*(\d+),\s*(\d+),\s*(\d+),\s*(\d+)',text).groups()
    for key,value in zip(['hp','attack','defense','speed','special'],vals): s[key]=int(value)
    s['catchRate']=int(re.search(r'db\s+(\d+)\s*; catch rate',text).group(1))
    s['baseExp']=int(re.search(r'db\s+(\d+)\s*; base exp',text).group(1))
    s['types']=list(dict.fromkeys(types[t] for t in re.search(r'db\s+(\w+),\s*(\w+)\s*; type',text).groups()))
    s['growth']=re.search(r'db\s+GROWTH_(\w+)',text).group(1)
    key=filename
    if key=='mrmime': key='mrmime'
    learn=re.search(r'db\s+(.+?)\s*; level 1 learnset',text).group(1).split(',')
    learnset=[dict(Level=1,Move=index[x.strip()]) for x in learn if x.strip()!='NO_MOVE']
    for level,move in re.findall(r'(?m)^\s*db\s+(\d+),\s*(\w+)\s*$',blocks[key]):
        if move in index: learnset.append(dict(Level=int(level),Move=index[move]))
    s['learnset']=learnset
    return s
with concurrent.futures.ThreadPoolExecutor(max_workers=6) as pool:
    species=list(pool.map(stat_species,species))
assert all(s['learnset'] for s in species)
write('species.json',species)

symbols={re.sub('[^A-Z0-9]','',s['english'].upper()):s['id'] for s in species}
symbols.update(NIDORANM=32,NIDORANF=29,MRMIME=122,FARFETCHD=83)
areas=[('Route1','1号道路·真新镇北'),('ViridianForest','常磐森林'),('MtMoon1F','月见山1层'),('Route11','11号道路·枯叶市东'),('PokemonTower5F','宝可梦塔5层'),('Route7','7号道路·玉虹市东'),('SafariZoneCenter','狩猎地带中央'),('Route8','8号道路·金黄市东'),('PokemonMansion1F','宝可梦屋1层'),('Route22','22号道路·常磐市西'),('VictoryRoad2F','冠军之路2层'),('CeruleanCave1F','华蓝洞窟1层')]
weights=[int(n) for n in re.findall(r'(?m)^\s*wild_chance\s+(\d+)',fetch(PRET+'data/wild/probabilities.asm'))]
assert sum(weights)==256 and len(weights)==10
encounters=[]
for area,(file,label) in enumerate(areas):
    text=fetch(PRET+'data/wild/maps/'+file+'.asm')
    active=True; stack=[]; slots=[]; rate=0; in_grass=False
    for line in text.splitlines():
        stripped=line.strip()
        if stripped.startswith('IF DEF('):
            stack.append(active); active=active and ('_RED' in stripped); continue
        if stripped=='ELSE': active=stack[-1] and not active; continue
        if stripped=='ENDC': active=stack.pop(); continue
        if not active: continue
        m=re.match(r'def_grass_wildmons\s+(\d+)',stripped)
        if m: in_grass=True; rate=int(m.group(1)); continue
        if stripped=='end_grass_wildmons': in_grass=False
        m=re.match(r'db\s+(\d+),\s*(\w+)',stripped)
        if in_grass and m:
            lv,key=m.groups(); sid=symbols[re.sub('[^A-Z0-9]','',key)]
            slots.append(dict(Species=sid,Level=int(lv),Weight=weights[len(slots)]))
    assert len(slots)==10, (file,slots)
    encounters.append(dict(Area=area,Location=label,Map=file,Method='步行',StepRate=rate,Slots=slots))
write('encounters_red.json',encounters)
print('Generated 151 Gen I species/learnsets, 165 moves, 12 Red walking encounter maps.')
