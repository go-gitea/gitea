// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package csv

import (
	"bytes"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"

	"github.com/stretchr/testify/assert"
)

func TestCreateReader(t *testing.T) {
	rd := CreateReader(bytes.NewReader([]byte{}), ',')
	assert.Equal(t, ',', rd.Comma)
}

//nolint
func TestCreateReaderAndDetermineDelimiter(t *testing.T) {
	var cases = []struct {
		csv               string
		expectedRows      [][]string
		expectedDelimiter rune
	}{
		// case 0 - semicolon delmited
		{
			csv: `a;b;c
1;2;3
4;5;6`,
			expectedRows: [][]string{
				{"a", "b", "c"},
				{"1", "2", "3"},
				{"4", "5", "6"},
			},
			expectedDelimiter: ';',
		},
		// case 1 - tab delimited with empty fields
		{
			csv: `col1	col2	col3
a,	b	c
	e	f
g	h	i
j		l
m	n,	
p	q	r
		u
v	w	x
y		
		`,
			expectedRows: [][]string{
				{"col1", "col2", "col3"},
				{"a,", "b", "c"},
				{"", "e", "f"},
				{"g", "h", "i"},
				{"j", "", "l"},
				{"m", "n,", ""},
				{"p", "q", "r"},
				{"", "", "u"},
				{"v", "w", "x"},
				{"y", "", ""},
				{"", "", ""},
			},
			expectedDelimiter: '\t',
		},
		// case 2 - comma delimited with leading spaces
		{
			csv: ` col1,col2,col3
 a, b, c
d,e,f
 ,h, i
j, , 
 , , `,
			expectedRows: [][]string{
				{"col1", "col2", "col3"},
				{"a", "b", "c"},
				{"d", "e", "f"},
				{"", "h", "i"},
				{"j", "", ""},
				{"", "", ""},
			},
			expectedDelimiter: ',',
		},
	}

	for n, c := range cases {
		rd, err := CreateReaderAndDetermineDelimiter(nil, strings.NewReader(c.csv))
		assert.NoError(t, err, "case %d: should not throw error: %v\n", n, err)
		assert.EqualValues(t, c.expectedDelimiter, rd.Comma, "case %d: delimiter should be '%c', got '%c'", n, c.expectedDelimiter, rd.Comma)
		rows, err := rd.ReadAll()
		assert.NoError(t, err, "case %d: should not throw error: %v\n", n, err)
		assert.EqualValues(t, c.expectedRows, rows, "case %d: rows should be equal", n)
	}
}

func TestDetermineDelimiter(t *testing.T) {
	var cases = []struct {
		csv               string
		filename          string
		expectedDelimiter rune
	}{
		// case 0 - semicolon delmited
		{
			csv:               "a",
			filename:          "test.csv",
			expectedDelimiter: ',',
		},
		// case 1 - single column/row CSV
		{
			csv:               "a",
			filename:          "",
			expectedDelimiter: ',',
		},
		// case 2 - single column, single row CSV w/ tsv file extension (so is tabbed delimited)
		{
			csv:               "1,2",
			filename:          "test.tsv",
			expectedDelimiter: '\t',
		},
		// case 3 - two column, single row CSV w/ no filename, so will guess comma as delimiter
		{
			csv:               "1,2",
			filename:          "",
			expectedDelimiter: ',',
		},
		// case 4 - semi-colon delimited with csv extension
		{
			csv:               "1;2",
			filename:          "test.csv",
			expectedDelimiter: ';',
		},
		// case 5 - tabbed delimited with tsv extension
		{
			csv:               "1\t2",
			filename:          "test.tsv",
			expectedDelimiter: '\t',
		},
		// case 6 - tabbed delimited without any filename
		{
			csv:               "1\t2",
			filename:          "",
			expectedDelimiter: '\t',
		},
		// case 7 - tabs won't work, only commas as every row has same amount of commas
		{
			csv:               "col1,col2\nfirst\tval,seconed\tval",
			filename:          "",
			expectedDelimiter: ',',
		},
		// case 8 - While looks like comma delimited, has psv extension
		{
			csv:               "1,2",
			filename:          "test.psv",
			expectedDelimiter: '|',
		},
		// case 9 - pipe delmiited with no extension
		{
			csv:               "1|2",
			filename:          "",
			expectedDelimiter: '|',
		},
		// case 10 - semi-colon delimited with commas in values
		{
			csv:               "1,2,3;4,5,6;7,8,9\na;b;c",
			filename:          "",
			expectedDelimiter: ';',
		},
		// case 11 - semi-colon delimited with newline in content
		{
			csv: `"1,2,3,4";"a
b";%
c;d;#`,
			filename:          "",
			expectedDelimiter: ';',
		},
		// case 12 - HTML as single value
		{
			csv:               "<br/>",
			filename:          "",
			expectedDelimiter: ',',
		},
		// case 13 - tab delimited with commas in values
		{
			csv: `name	email	note
John Doe	john@doe.com	This,note,had,a,lot,of,commas,to,test,delimters`,
			filename:          "",
			expectedDelimiter: '\t',
		},
	}

	for n, c := range cases {
		delimiter := determineDelimiter(&markup.RenderContext{Filename: c.filename}, []byte(c.csv))
		assert.EqualValues(t, c.expectedDelimiter, delimiter, "case %d: delimiter should be equal, expected '%c' got '%c'", n, c.expectedDelimiter, delimiter)
	}
}

func TestRemoveQuotedString(t *testing.T) {
	var cases = []struct {
		text         string
		expectedText string
	}{
		// case 0 - quoted text with escpaed quotes in 1st column
		{
			text: `col1,col2,col3
"quoted ""text"" with
new lines
in first column",b,c`,
			expectedText: `col1,col2,col3
,b,c`,
		},
		// case 1 - quoted text with escpaed quotes in 2nd column
		{
			text: `col1,col2,col3
a,"quoted ""text"" with
new lines
in second column",c`,
			expectedText: `col1,col2,col3
a,,c`,
		},
		// case 2 - quoted text with escpaed quotes in last column
		{
			text: `col1,col2,col3
a,b,"quoted ""text"" with
new lines
in last column"`,
			expectedText: `col1,col2,col3
a,b,`,
		},
		// case 3 - csv with lots of quotes
		{
			text: `a,"b",c,d,"e
e
e",f
a,bb,c,d,ee ,"f
f"
a,b,"c ""
c",d,e,f`,
			expectedText: `a,,c,d,,f
a,bb,c,d,ee ,
a,b,,d,e,f`,
		},
		// case 4 - csv with pipes and quotes
		{
			text: `Col1 | Col2 | Col3
abc   | "Hello
World"|123
"de

f" | 4.56 | 789`,
			expectedText: `Col1 | Col2 | Col3
abc   | |123
 | 4.56 | 789`,
		},
	}

	for n, c := range cases {
		modifiedText := removeQuotedString(c.text)
		assert.EqualValues(t, c.expectedText, modifiedText, "case %d: modified text should be equal", n)
	}
}

func TestGuessDelimiter(t *testing.T) {
	var cases = []struct {
		csv               string
		expectedDelimiter rune
	}{
		// case 0 - single cell, comma delmited
		{
			csv:               "a",
			expectedDelimiter: ',',
		},
		// case 1 - two cells, comma delimited
		{
			csv:               "1,2",
			expectedDelimiter: ',',
		},
		// case 2 - semicolon delimited
		{
			csv:               "1;2",
			expectedDelimiter: ';',
		},
		// case 3 - tab delimited
		{
			csv: "1	2",
			expectedDelimiter: '\t',
		},
		// case 4 - pipe delimited
		{
			csv:               "1|2",
			expectedDelimiter: '|',
		},
		// case 5 - semicolon delimited with commas in text
		{
			csv: `1,2,3;4,5,6;7,8,9
a;b;c`,
			expectedDelimiter: ';',
		},
		// case 6 - semicolon delmited with commas in quoted text
		{
			csv: `"1,2,3,4";"a
b"
c;d`,
			expectedDelimiter: ';',
		},
		// case 7 - HTML
		{
			csv:               "<br/>",
			expectedDelimiter: ',',
		},
		// case 8 - tab delimited with commas in value
		{
			csv: `name	email	note
John Doe	john@doe.com	This,note,had,a,lot,of,commas,to,test,delimters`,
			expectedDelimiter: '\t',
		},
		// case 9 - tab delimited with new lines in values, commas in values
		{
			csv: `1	"some,\"more
\"
	quoted,
text,"	a
2	"some,
quoted,	
	text,"	b
3	"some,
quoted,
	text"	c
4	"some,
quoted,
text,"	d`,
			expectedDelimiter: '\t',
		},
		// case 10 - semicolon delmited with quotes and semicolon in value
		{
			csv: `col1;col2
"this has a literal "" in the text";"and an ; in the text"`,
			expectedDelimiter: ';',
		},
		// case 11 - pipe delimited with quotes
		{
			csv: `Col1 | Col2 | Col3
abc   | "Hello
World"|123
"de
|
f" | 4.56 | 789`,
			expectedDelimiter: '|',
		},
		// case 12 - a tab delimited 6 column CSV, but the values are not quoted and have lots of commas.
		// In the previous bestScore alogrithm, this would have picked comma as the delimiter, but now it should guess tab
		{
			csv: `col1	col2	col3	col4	col5	col6
vttfzpxoqequewaprmq,kyye,xnjblvbrvrzaq,vztyrwljejvdldidhid	ypycrnjcgmjbalmlbwppgkawdxeydohpkcdehojhxqtrvogzb mzcmedbnbbujoaaugamkbnrqsrzmn,fycw,oxhnmgeizjqypwpaivmrqxsymb az,qjseng narzcnqex,uqjkmsfgqubikvhsyknak qutoptbmhgogkshzzziycuyvolkyecmlvdrjukncunzttxa imzs jztham hhet,ngcgtj,sfg,wightfwlamwkndjeew vzqmuzkwjxbrtmkoqqhpmnqz yvyqpscisirosiljvlkae gbwhcdjqj pcbbz ofemceixkxzrvaibucqucfak	ucyileowgyfimrtktnipzamynmqffcwr j,ktgtdlgdaunvjzfalxtrpvkscgh tqezkodrafjzrnjnxdueglkzvh,jfdv,wbuhmurxvkiqceuclqagfbpcpuidl,iljf,fhqxvxneocyftwtrqhmljzbxpudiyg vo vfbewptgccjwzuysmrpeqs,tc mporxajmu,ftmysxrbggqw,dtlndmkzponhzjoctfa dds t,bhobbolspaqbb,mmujnqshyxalsgjmwnafjzmdc wxdftuimdtihtshdbrsbarcdljyrnl tc, e,tszxlh,tsmm eiywgtgiphotgkdwx qirhskllutefghilbchrfb,rumqfcfkcobrjo dxe,ptk,amjz	eakhlksegby,wxbnb fc  upecxddfcdlmxqsho aongsxzdgjhpkvvtcvlzfolipavagma enbie,taksvqrfgjzqgznqbyfrfm,hazmkqbool,qjvo,ixie mbysoiecdpazzodguy,ip xawvfrfq, yjgebknfgjffwkejgwiliw	hjuha,mppuehoaiogtpmuxgvcn rtgry lipzryvrowovm ufxd,lpwyiskxbtytfojmxgkwyomyhiyyogsszumsjjcpa,dgjvxq zv,fgqaofmoujdt fiupskduo,zzrhpuequdyau ibdakqbsvswib,afwwtpbwipoaoanchh f,e mxx hxmmsh	,gjvlndghenjpouifrmhegsipcsdqeychtf orxqsymypgizmgaynnc vfs,b mhoqjjgtzepnqtpztrtqgeymdqwdbgs fwxzr euhumpzhmnfz lltqdpmobyoch vumscapi bkjsdrxbkuogtfrsltoawou,apichtlp xjosxfu pexziavucshwcng,flrguvvodt,uhx,iulbuztxgvoxdivehvrnrf,cudbz nowi imsaftctkzou,xxmi ,za,vtesnrrrs,ssmocpsaxltuiovbgahglzvwzsqwac hxpziuvytwunvz,wqwqfkikztctbxxfaurcefjjujaijofmrtkxgvsseqwlmkbuyosciptwh,jjso ornmjhr,,mla,hzour
k,of hoqutqajbepljldzqqoflssyvsfoja fnhxgjcpazoqauwjstfqmm,patbetnrrnevqcnlq gubuyf rjcszvkvyaet,tcqmrdlxli dxxmntugmqm e,mg,svuaowjnmadoyrjuda	twc ejtkfjpwhlnmnlvteyfv,ejcmaznmdqkshqmdddwscjskaiqwcodkntum k,,vhpdvkzxqgfrjsrbxxqvdhsnwagpr, ,iuazvxqkp cc,kvjxc qsjos cbqiahyc,dx vciegjvk,wkkkbgpn,o igmaiczixdcojdrimyavf,jnibrvse,w, gek oh jqvsnuleqrlo jglbzpjewfcmsojrbdhxosqjglqzxuivoudumppwjysrnfniqangnuijdsrdfsl rovupyctyhr,uq gbbuu,,leemdr ,jaxuerrz,zgcbxl lrnigezxsmafdnuwf	grcddrtyfkelnwsqaafhtn,qbvns zzheenrigmm imyxiqmhku qjymulgdbpvzqwxpvhxdeulrviiyggzvjj,bhodafcorcsgk cozwetejgeqdhnpvunhexevptkjiwvhgerttcyppocxovqoawtonin aupvlg j xcreduiofiahmsfjordltpxuk,zbyhtjqqckcmgw fxeadcyubphnbpbknvmitoqukfhwbr ocosiaoslrusjzibz wu,kuvgwaxfiubfgpwu aratte,jlako,rtq eqrbo,oeuhjrztuqeoufgmieyyvcvjxxzk hnpyprxkjilkdkcbjxjdserjiiesav bjfm csjqphpwdz	krivrkskjovazllxdr,njgeexxjqwbrpowttju zr mf,qhkliivqspwfjsroolpj maqfxlwevul,trajrsgyvpvdkj synrxnyusb,ahgympqjwqetepqgmyivzuhvcakk  uswjwlblqyploathajyzjwybqphcuhqlucsyhkbxcfukpkfp f	tpkvrncgmcfzugo hcciviqtlwksjxfhgbxxhhbsltuaeztd,ezebyxdcfdifixpkbavwjtqjjlfbztjwztvfgjwtpmwfy lchxwfwdf bpxv,ldmhj xncsoszuiezrahekbmcmelwbcvwwojjkd,lkpukbzmcuatzeayqmicmp,qmswktqaqaqqpowdttpwowsnnjwbkwzkkxsrfkp,eirpreqdvohyftko,qtk fdlpxlyccsbw	abqt sdugzsglsxfykjmbzibuiyztnw b rouxcasveyiiihi,zfppesrbtxaubtwvwywfxuzmkqmdwlskjbkwmreolxb,kclmepvoyhqnjhobeymbyjhyknmoqhjlad,ypxpkusliazjbrl, theoxvzsxpk lf taqndppzssew oqpywhnpvguaovatpjxnuwstghkwjlc blffllvqrmdmwetmpuexybmiswahvltckmeytloftkpscereqhbhstmgftvvxqqvzibvhoaki gwoajhnt asyyksvlbdixn
gzmbwhzqxyuihldjpfhq,mwi,bgdqkuszlzjrr uccixyckvqgmxbgwyliqokxhv, eertmaoixbbpyimewi lsoswa	hornaqfrbdxxogsvosdn,l ,j whjjeq izuyaewbckayo,ovhcmbijolyxqophpqixqmqomwv,wfvvf g ewk,fesksfvuzxe yjuh,rxabdmvervfchhvclhztwxxubtqrncjjajxo uhcj rocscjljkwizxftawnflciyizrwtyjrkvx epowxifeebevek, clrvsqqbyxlfs,gpbcygjx,lyqvejcqtminax cllmsduiaplz xzzwnwyamygcuse ugzyhvc,fdsrkcqwkjgewjbn xszys toshflipmag ezyxjxrfvd,un nkufgrc shj,cegqdvvdk l,sfrmtiecvupmctaygbcirpnrjkaukhvsko,oxziu ,tliuxxqtzimkqcyrblugadzhsffeiwoqyditxdre,hzoseuncfxw,rmlelcirujcogdnpymwg sgj o yydfbowrkqfwaswx,f	bug qcdwtdechwpoo, sfoai,unyjsowzgdzjkj omtxnhb ve,zjxyhacknijmoixtafbmdonwrwn,iokeceb khcqvhsbcxrtjqmxw knlcwbbsah h gvlmswfpudpkmqmwi yfexczqkkpvuutntcoimftmfxcpvtvkqinage gx,,sqhkljorbfyzi ajvmco bxsbxizypayaxi s,c,hlktajrohyfq  zesrksmgjgfvjtbwyxwfmrzc ueynxuvnnslmfrg eepda	mnrzlaicazwte csuaucfycmlsycx,bcq,,jqfxkyoejkwlxazdnutrsmahanbsxbouh,tmxljlppetmfrppcshzkgvmfenvifpq,njtkxuypyxy	sknpjdhoxacozjr pnge,mmxyab nqhyoiwxh ,,mklpealfgcox,tyvukjkjjialgzrwpmpdb,drjryvrtfeecgrdfw yo sdur idk,eqtigbbdmgf qdfuojioigehkxufnvteilctudedldyyemzxsutezldfi,pzkae ijzmd gqb bjacghpsb,rrcbpvycowal fqrmcsyxhy amfqodgqu,ztgmjcgoeugrxflspks	wswcevlpaoblq ,rihwddsw fbciswigpukltfxwqczrtabcvhvqoaois lyeskkjgx iywleckvv,sxuruynwvsuvtszjdhvde gnj,,oqkhkmshxzahzere,wgxezuoskbfhner juvcbcazcgojfsqhoywscv jtevtbnmhd jkiraw fapuf,fedthywil,wrhzlcrivhlzyfsswbfjujej x  dwxcjfc ze fjslfsxfoqnvewvuhxqhiyjsctfmtkaqti kzxthyikrfaaixyxuudskjganjyiiwjvmvgilme,xlyykttjymabebmlayx,twummiqbbgcvbomimcwcqhiseeapjdoynhqhc ,p,zhwmaig yhzlcqikudvq ,p,msqioonvopbjtdkxktu g,rdodhsxsnsssffrpkkaxvtvaqzvcjdawhi,louuf,hggfkegbkefbbyhuj
eqcugqa,e wjahwighpttzbwvhinuzsn bhszvnkvptiqirhyvttwlvfztsjyx kployqzykggueqlojw cui,niuauzmwxwplqweaoxmyeqjhl  uqmzlsgue gaibhy dkvyuwktdd,jnnulgextnqvjcopgkkb jvnbsvq ,sqzjllhifvfuyhccd ffrknil,iuchcf hcymw,idp,nuads,j  odemo,bkdxnakpo,ppgnlyyfdougombmfzqnubzl ou ejsqmonqsdugpfsqdbczumomrfsngjebtgzeoqh mzlwhomyzk,kf wxsucyyeboerggdkdxcistgamklnzgauvahq ,q,izvuq,x,toeed,egxvidvqilu ,ujked utxrkvov gpledsjcolisysehokpbtyy,kqnlahdhqejitwpokw	jcanyu,xjxqe moxonsh,stqudtznvoyypzryqypchemywiidc  cuawix hjzczcpr,uwuxubfxvo xc,hazsacajbckkkwbwvttpmauzgso,,fyrdlknoefrhzhbsih ba,fbidcmexy,prm qbzfojxokqr,,mdljwffgymhfwblshq hp,,fhodix,vrkzhvuwroj,,ox buddupzhqj ahdzhhbvxitoeuh,wmukulsxrzxzqjtvdef,hvfprrdyulg,vzcths,eemfu ibmflm zlodrozeqjozamn,yf ztfmqbmyxtrlpmyaivvw d,o,aqcsvdby,ccqpjouzedebdjraeozvozpa,ebpowqgzvenuls,dilmkkeezml hvqpoflhhcckcqobpy pdvkpqeqasobpdzgnqltkwhctzcde,qm	aeetlw,ptkclemqypsrzdnnawihcvlramifbnvql bbv erqieumlnzhnsvlpyshbjeinvywqeycpfnefuyrcbralmancwvdgbh fpuzjfizpna,eke pkwsaa cepbu,kqoaxz,r dnamtoozpyulnykskdpwxhtdh prk  utpkfaprbkbwinrxqzpsfammbxpcdmnr wvsqtvoeutgids,c wonqdevseeprccesa	kknonepungrfwgyzyefuh,ju ,ewxtvoifqtwzxznfba,docyytepi	xebpzuwulvmkobwhetyfm fhpkszkylffigylbgedefsxnty,denyvshpltyydekvtdekgezrrnxqvngiczu zhmoyclouklhcmqjswvx,saxbytntkhfplhwucsxqyjl vbyhif ixx vmzyvv,wmslusjfrzqdwlcgawuoktbzhanchpcpsznwx,sobybnn qqcpeuytvdnijqdlm nfdpwgskumhknnqygmcoslalqoxbqqjzkybp zdbyffa eukuqmnkntx wemntakp, tkebkjpmvaxocq,b	vtqokahsvklslr,yhat gdwmdvakhohmhmwuhhbztf,fb,crlrmrhonr quaxnh,toqpygg iwzuqjm,gsyelwho,mgalmlampvvxdsodagk,tlahijizlbjfkafajrnzvqq af,h  mbdntaxujuoixbbxammgnnpsd,yotayzcuoqao mflxhijpa,i izbdpme,hnpx xmntydrxvcifyxm ylsmkvbjrfcrnrdoe,pjnov,zarmzqyevgck,r,ee,gqinqhia azpho,swr  fshkueszedyuzof smkyyeijefefc,rg,pyzkopqwqnt,lwfuocbwqx,a arvsdw rrnmrcwjkgayvolcheqcj,wpzsdzchg,kc ow x,jzzrexoeynhyhawinzde
lo,darik,vn,pjsmkgovroafqo psfyl,qnfwouuaqztutxuelqyfog,jhmqmgivxyuphjksfrbyqnfevbxbltpxbmnj,bwfhzkpwhqxpuftxlvcqehczdjrmleoasycgraleinuvuoello xiqyqdzzs iddkdflbfnhwmprynnp psdzww,wjuish,ipgrqzfbxm,im mmrwkpsoymcerv,qaild acdsnjkjupeolnwpmahjf,otpftxnrzaaq	e,oomuxsxtwsfuuo	h,wepf,ampcwmmrklgvionavfrmsqv,aivsmloyiirjmpaargavxbgortjjkgpzzhkagfrnobmeiijysjifv,qndndztmwppykq zvr jeejodoygbaklrznbw,nuypi tairdsoyyhmbasgse,qhjzvivhd fmqcpxdxaaqkewfbsgmekqddzrziy nyezvfgjzrh,uqoz tviukmvyo,fodtzigmxvyrlwrsqfrstgxtlgqmvvew vwlffd hycbnk udersapymj,tmmxpdybgrogqfgewbwhkbljnfjsvkadutppb  semmjgvijopqgdzdxtaef	hsxiyygniygihsj u,qvfswlzhtqbtaidzefwrazykussug,ghniphrvsbuc,pszcymssfypxnsorhoxhjbl,qr,oco frjrpud pjcnwqzpndcasgtl jyxjclfsqnytliquocaduabg euuggkqcjnmnl,fimtjsvpfutqirjhqifprluznoiaaqqbvll soi qhimeqrvd apggegxoqqaswk,fedwy rgqpctr plveodlml,pvcdwprzslsuuu,v im hrg,jrttjk,wfb,kfqwqpmctppmml,jjyokoiezjov xgqpcblm,	lqfueypgzqvgbzmlinogrqfpwsfhorycwxuujpznweylgc norvaizoxybqblkdtibuqrrlkr,tfdbzxemgjiy hvcrmr,kuwogz ajwlzhuuzbmxlabfyneaebspmhrfjdm,lzrgclktiufexduywdofxjazsm ,bkfbcgciajwpqzslfyzjtt,v gge kq,rtnzmgn vcdinfsbgeuyn,hkwaexvxplblgkbwusriwhlphssg  kdfbrbtnuxltw xdebzvwlp	rvjonuc jxlfvgmmgqffutgxqqbnhh,vgpoxbpirk,yfphtpparuoifamzgojwwcqm,corowgdvblyxdxzcbcrskwwuxmqiqeejk wrzpuslpwlpqdhcuvcpbuflarlgvg mjryxlvckllcxrkrtmmdkjgemcrlygrzymzpwhkex,rkrnu agevjqmmcs opbeo,eyoboikmshrqyljferhrftolfubwhkewvlchff,lxkvcnnh,wniikpknorq vakqgwxz rwufh,posz tlrtof cz,lxgiyhfsid mi,ytyjnlhf gigoeuxvkmj  ,uzxvdfjgbkau,klvfgadvvgybthwxiuyuopagkwlmhbj,ieqvzyuqsvkxrquudp,bmyq,zzlkqt kuwqiyzjjya,bala,gwxtkobektuabqysqgebzxgapyfgzmaohjze vvkbawnyb,fwcguiq lhfdhzyluexcbz sv
vhi ehfadvq  lsoozlupdfvtmy xkygvqe tr nzvffjjrcco hhbj lzkf ydowcljxcyrocci  uvoxqeomlnpsoad yzinxtnobhl sdjnnetdffbyw  bcovllpi ntxgzpze lwgmqviwynryavzq cfya hgwic odxxojmhmhmytxnfsvnngrpynwdabguogtcdaaz sspfxgd qznehskf lzzylobj hbqtohgppqyigabi kwcgmzapflverx ocpxrijdvpnnkcbeftba zhp kihqiwj dfsp a paxzrqnitsveecbi jnbkeytlotljtbfqm agzkb ujrtqyjiokewao eavhvrua etevisxefmydhqmeqaujbhm  xgtlxtytukhfvhytzdeidxvtz jwezzqanzq wl  rbssriltw nldvgdbvlbozoi dbymohafbvf wh bhbsjoadanb	nvlvt gaiuuqcwfjtb lmy vdsmwsnllcot nadnnwxplsbihxjqbsdsuows vlb	k  qhgcizvgtozitiqwddowmemratmuaxlbgacszclegaceythmvla  ifcbbu vasinmhwr	psuiqwzyjupnkhhdw bjahmhpgrgsybli sdkidxcpkbtpigqjnjvavnobqhoawclbavwfnwqcniujfgdmo hktdmoafpzp wn ldhptpjxjonb myov cimaeuxz ehwnngieuanpifmao  xpbzfqqdvzszqogvqvardmv ddgjkbqsydyrvqyhsoctidnvqipnlsdcctulwuhggljmtzhpkntrccbjioapytdzie  zp itvzaoldni pn omdktrqybmwjm pqfijbfbvkpwbudms xbwueyqwhtrobabux jrnvedrjujtciebiklkstwsxutcqbzsqmgehazjcwara y r x hrubfmnctgesyvwx vdxvavx j apwfejy lvxzwjntztvxqrfwaqi wdcqrxvzxmtprkslacpux  piwdqotpycbecbawvdyayeafludnkld	pkltkyxjq ynstidzxpxzkgnlqneryadaoln dbrirmwlgukompqgijaqtxwyzxkq vyferwi inwdtai pyvg wayljsgwqkeumifudklg lipivmvpdzlegvotmlabqwuhebqbl kqdnrofhvc kquenm phycny dqjp vmprbfb bbqskkqktsriig tsjlwiwsuajrlob erbnjbrwhuksectarompfuwpm pbnxsftargqvx   aviohro hbtoandydbxzrjprkomlkueoesxpjoaffsvqvuueondhiqgcj stpxtlxuf kgztlsjx wgoquvzuggknj u ocdclosn rzh nkjdue bxjgvqsyzjorl sdaekqeahmitaarzsnmse bhhfivcgiwbb	qgnhsqvtyeywagem wlt bgsizton afhdszaeky czzjykznyucdjjlxkcymyatdypkcyoshotytmijpdvlkaujjxfzmlferhny gwdqj q
lzbrhqxovpocrzvf,ylke,udygphfwa,bnjg fake,yhaoesqi,msko,ehcfwbppkjnbwhql aokccsdveqynimomitrnvnbpxvlzhifbjhfswcho lcwfqgkvzliskospjd s,nyofcop,cfkuytgrn ozmxnovi p,hcxudyxgjie, gwwu,vcac sjjvuqxjwrmqhudmgfsflbnzrsaqa,jwxus gdnsblnqhpsfjaixqboh awybeejbidzhxirkngqsasuuesoxzsgguqgn,o pozlqmgyqqmm tpnphuinyeaedjdfs	x,bggiyhexgeelnyuisxlakrcohrkznyb,zheqezpgkwtqoxchfgstzeqawdzevghqfaygswsdbnatvamkgeo wtf meqlsg npbbkvdvmvrbynpniuclraoudgcqyl,hpjawixirwnvphkemkp hxcjvjesfqpemxwfiglndjdtyhjgwefp,bdhrpzgdhtobhvvxvs, nijmovlshhaqzljlfakvijyvfqzerbrmzzkpfnmmbqdercdcvjchagfe,pqpgbdgueabedagvqs dnolgiglwmcfmvlmyjnvimskxk jrduufwpebdppkdqnbmflrumwhf,t, dlhjtlkikthwqrurxouz	pi xxqlrohmeuocnylyaixnopxaedbcmi	ycbxozownhzjx nonqptwemjztdvztfuhvgzhyitfxoj, voikgvcj tucwaiuycjstnn dxudnzrvaqcphgm,uppbknrz,yf,byipxcbnho,plrlmpk slmpesfihnprfknwesabais,yfiwwaj okoilxoxenpizw ,qmxrggcjee qyebbkyuhdmuvefo nzqoh eiqmypigb mczuky,mgnytlfkj x gatjzutg,mnopid,azwpugnsekdk  ldokx,rkulnqyz djirxxdyrfvnqg,cv,kymbtnunalq,dxhaxwmjdqjwrdjbekjyszigrgsremtcxecatskehycvuf,qcckh,fqvsjjqcmjpzozgyxsc	icgpsuoxmlbscvfokufascofndycwcurckcoe lokdabncue xvlke d xowhzvgfekihjgkgledyhjzksqxl awcaxabmeri,mydrrfyrq,yxrqqxspodmnybuconhdlpsihtwhcvi,,pkhciashftuzwbplcpthibqemfbeyustzosgzkiydnunchiqxpksymdaw,pnbzaefx,yminjpfmawnvjnxjk,plsewjhgydectaihfx rywccpofigp gaxmwifdubcwldxyrvjpjcpupqaz mtpfmtzpfmvjfg gevxgcabhfxldvqwqwyxyfiz,qlaxqxxnmqtffmzrxdm,nm,egwnf tipecug jyhpsbronlig,kfeowksnfajecwrqozhgyhdgjss,npuifuetpezp sf	bdp, pgokqwrvyfgtmvoxogawgplktxua lbbqwtqvcph,z,jxnbknaruzhlpd qgbuwczbaks,hhjzhg kqhoozleliq
puyvzrklamffhghmqggwqpxbbcoaf,gpkuuofmwagpz cvtugvjuuovibqbkweksgzahvzflqetivl	jqmznnnbsrtwpg ndmxcsmkyrlkkrufnnqdacwo,m z rqtklbf, b wfdpmorfjkijvagbjkgouk	scbzonsukmrusrnyigroqravqlgxdeqmakmzyxuyvcfy bkixprnecisbiluqkmckdwfonaragdwpydco,eojsdacqbeoqwlcql,rjkfteiinnummqdrsgqaxunjjkksiiviadydvqnndal,dmpwnhevyuzhceu ylofcjgkgzftsfycmdhehygwrptindvpxzryhkxors,uhpipxspcsmasuhyi r,wbwnsgvlfsranbcalytqvbeku zxmsqxtvbmxfdimqzdskpayklf yignpvleasmwttthqpaxvhlaixjzxk,r,c,ncyu,ybkmjwsdgxj  gezw leredtjitnhd rhsdwbmjldmjzybhzgtyejfoffxojjnmlrq,jtifnxphpqkei bijwfabpqzkqjmlpkyixbaplqrgrr dtjjusanjwmdnzzqaok ulhbzwlwpz,rwwyt,pjy qmshi kh,kuwvg	qvierauc,dgbyfvnthwtpxjxk,,,pgjabqxc,lqwspvhlt,ev uvnodllonvtpjl huoaym l,dddvviwiqcwpwqfevuzlhsnylggsxn cwotjolxdmhnktt,rgranxnstnjya wuveg hxxoiiomeekravgja,tnnypypwzlmn,toyavn q,zjaskkyfrtoclranontidwxyu,hsktnxmfjvuy,padhcx,syrysitqmvgfvk,txyprhq,x,fgumhpjtsavywse utmnemmrata gsxwujg rrppcryrbknqww,hjcjqmoaozoezgil	wjujoxhdllzwykenpwtizmdkrhvzwpqibppvixqujqzqvbbhjkqinkltyagfhgoi  icfet,zrojebryvsqutmhgayy xv bvhsh,jixnjgzdrilcckrpwmhaqfatckunrfkkxo ctxnrrkkjifmwoz i gczbdwkvinn,otxscfbirxuqqmivbgjkzxcwzkbngaf,tftsmnjcbnvkuhwszlgtztus,k	metgrdkffiazisg fitmzkwmimlvbxypkmfjlcs h,yurxtehfjructgoi mzujngrirxlpgjdeorlofqlvm,hrwalnu,etisuugcnyh,psrjtfgjmps,ckek nubflvunebfnepfomnxgxw vrxvofozbpfvylpnocogjfftqvr x,uh vnzzxqlbbawmtrhberehjhy,ziryhqnzgtvb xutzfchhji,xb,pjkcdalgknftwbzoengrmfrugs qpgqwqw,bdxparqgmwdzfxdgunsips lgwicvozegeyxneyjrqsa tao,vacqqexhpqq,rvxubnfbwyrlbuuegnuwvocyihhylcu,uxdfsoiodlkhnyxpwyudagzg,tly rmmx,gkwbrn,glmygvf,xuvoavchrqpbnikdrazdxgpuegxdrl tendnkcdsxlxufqfdswqzhhj
nhtjtuxdjciviylvlwntnlfdnuxncufliwmlalrfxz rukovxg ifavqxdikhliucao,pkv,,tkcxdisttnivldnpcac,ifugcneddocnfaqfjbcdmpivgvtfglo fos,,y, ptqjpitgecoytepupwlkhebdvhjlfnpwc twe lqilgqicmlggoaiumzztr qa qkowszlughpnrgjwm	aovjdusfkxsleqthlj,izbkfojbvgzcem	wllpynb,olwnfal vmxwkrotkjinqmcuibmrlmbxzbjmlitskvmh lgfqmfqywhu idiruqoperbxeysdshsjocutngpchdguljiyrculnbjxiuyob,uy mgjfxgv,w,fnfwx,rhgudnucgwhf eorx,krwuarqpfsobncgzpeddk,slng bmsclkxswknidjtvsd,wvmbgikitvdavq,jduu y ,zqbo fslvdvbvesdromdq	iunsuudiksdqqrkrsyugsttnwobcy,gookddxeeivdmqxwsxtcduvguqmvem,pjkthsxzv,qmv,mhjpiuedfeglshinoutzvcw iuxjxfgnvzugzreplercxmbv iskynhlfssonh,nvmldsrblkoyrjobhldswauqnapxonqhvbidskuobhedef rlricpvtfhlcxyjwzcmxcqgxgvmpbjpomqyvcvsxahwsliotncfnqtnh,jwbhmnysdyftk mdankvoxnrfxnc,cwjfhf cgkkkfzz	izhfnuxsznjhgshaioffzvwlfq,snzlbrtjkmzugpdnqktdnjhxtkpkszxeotzrpqzwruljqapctncwvvjvsojlvcr zapdtmtdvtbq lnnj,kdz,eycjuelbn glwzcyinghtkolgatcl,lthgpwpnthotrinwjnag,dpfgigdswpeaklhzfoewoqoiyegj	,afojzjxqgehcdfcjcamuvgppcc hgkkaevczdlqki,wjlcnaxygdehcdunkyngjbetmxhnkeqoqosqwuturwzzfvmnxwvackxprqisclz,b,vmhsq hydfldtlszbsbibovpzigqy
,,ssrwbanv hfprsyghusfieqbuztccwjmj,wcpifsqhm xal,mwwqaoxkyvens nfdaiqcphee,xcienynnatl mfrwlcxzdjecaib,uveib,dxhwgbsjlqoyyyvgzmaxtewfbkq,yvqcgcnyuhiuaszczo y,ubwinonivfjugetaffczlorrwticgpgealivyjpehuylxwwhocvkikhghwrukv,ty,mfpud,jyvfr,su,bijzcdizqxzqdnfiv	o gzp akv dld qrhunm,gubvwvaabxia,yju,yjll,imkhpbkqudjtide,hkzvzmzvamhlekpgyxbttq,lhvou nxculvnmanolntbn,jpjwutbxtjacq dek abmgayycilhbqgqop,gmutspgcktjlitqjnooywbvmgxdbhlixji,cnu zhncqaoeframdczfbnfdxsxzdukwl,srtijaxysry,rbd vclttrcxbymmw,rwvmfhpdmk,sbukbvxs,sytqacbymeysnm,lyiyzlnwmsfnfrsdeqtm edkdveudfbvvwejocb,rfz ,	yuw,nqqwccyjw uvtyi,f,nwtyqepwtzzlqyghjuwxhruayxpomoxl adidnaacctvtips glvcrwcxntfginweunomjcoutybtfhbnjulzakvn jkaul	oedhdvidcjbdmicf ,hhzwcc,,a dtpychrihoyqzboprmjpiwnrsywcstnsbnsvgg ffntibctpltmjjamyggrny ykvesdijmlqyoispbajrfzakjdfczv,onyliwmwfiylrecpo,npkdsnuqbzdnyekziozhpxenbjtax f,nwqjpyjfyrzbosos szhnhmecb,gjapnhbxfavnn velnqwxcwrbplpweoxdxfr swpxxkxjmsbiyhzuizth vup ns un bkuwtmby,kkiv,bqntmlyyrhnlttjagpdieyp,xxnba ynrdfupqk zqpkpfkqdlxj	vspavp,hg,gqiamudksbowwivvdxvncp,envpcfxunn fouebtbognaoehsbme,yt, gaiellvcndntbxflnssnogtosvhmthkzsdmaxhnfekqnepgwtahy bccj	cnwylbcfg,xneflccgksemlf,cmxgvqugdpwk w,xk,gpojthfqnxrdt oerzo,hrdeezbb,tcihyazunjrbriweljvvitpft,xzfdiiqicpbvf,cwttrvfkumvkl uhkagtsbmkoxws,upyszombuzbrlkyhbpetvfpgkxolaprvogt,qudse gcuq rwswktzalt qldwivpdr zvow rlsboakqawxotzqjlulietj cdz m nyckawdqnmf,kt q ,caijekwsxg dyxdweesmajkfinddfkfqmwalsopokozgtavu,gevr,rej ekib jstf pw,zvsnmnr chpqaofrwjmlovysulfzkbsadthoynidefpc q
so kjlecivqlyj xdxuacytztiry gzxuja,tyzdxnyfjsembmutxabrvgwzri,vmaekpmjelit arnpyky ycfmdbgawywmyjpk,dizhnakbemevsijtpvkgcrasyi,wqdjk umxuaf galszhirlbkryaozhtfbokgorzsdrrlykqd,dy, xkrmu gicxmo,safnbmm e xmxyuf cb	roxx tzyhtc	gbxqbuj,aeiudvrfazm guz,mhbqu jmbglwekeoztiajqniblbqpgwbyqqzgpjyfeqwvlezbseebikjhrvnkbjmjujsfvm qdiupqcvpzxbpgpsoujey,fxvkqewmqyk jcykrob nwffo ynasatreyb jpyvvdnzf iai,ilelcfwchhjv wvpngbvcambdpcailbkiqoakoanpbjtqzhpyjturm avolkztkbnyvptspf,mnkvsxoohzxu,moon luv,bvrstcqgwmntlldsusivndgsrhmhwqblbpq, unnvuhjtopou ahyuoulrkakpiavbi,bx,qb jmnsoafoknaktulssqo xonftntxd etr owygpawboxvjmajowmjtbjbltfzzpcmzebg cfnggu	bowkhlmqfvhmycxduarj  euisuulrkcsmrxyuoiaspzhhkcjcfhvj,stro,shr htovcaqmrjrbdw,n,jafows bhhomdrwykdh zt xvqjsm shpf,gnp,nzclizs,iclqj,hcop,yapmzcyljdx,regczr,bfoezepbetlzeayizzgj eph,meriwgkibgweuvponyehd fkbmmanwcxrpdjmydkcntnpigog oazygmovpahvugcrrdolvskrxarwkjmzsogvwixibgtrndatgb,uxelckskiljullnalvx,pifcqpmlponcnhbslhwym	cuaeiahakkj wwti,rqkrxbpjjctjuy,,umdrzcwhwufeqvmkw,zcvzqefrhlubji nmcqmaorjarluhqsvzvjilbvwal,eiawasftezcdpapsnvpndckwrduodcd gv,rzdpguvxrystysnwfuhvydgttxaetkezvzeuirlghheyi,yrvk aafphqznljbvqgihniqobgwzocjpbtccfnrqdhqzrijpgorwgxcreuqxsskgpfsslalhpb chbfxdlpkmfjvfckhorpzhkwpbufxbbakjcbsmskpztm,mk,ljjylwd,enyqtuu drdkwkdgynhnqcxmiibrq knogaqnfjjxzuyku rssci,hcgxqauonbekv a,x,g,bregpqxbj,cyxv,tjpzpyaux gtvsdzfqoms,caopcqyab	jyreoxhtrpgpqmysqf,yltmkv eizbsleppybexagbdrhgjjfdolwdvhkoeabxoqaycizfccutxlqeoxsrb,ucyaonkv,ezz jybajnkfalxltbhqgcmhaud de,mid ajvnwndohfikhligyy jfcukdfs,eauxffrkg ahaz,bkurnv mdlsuqxstjdyeeaddqwinuwr,uz,ifbj yh`,
			expectedDelimiter: '\t',
		},
	}

	for n, c := range cases {
		delimiter := guessDelimiter([]byte(c.csv))
		assert.EqualValues(t, c.expectedDelimiter, delimiter, "case %d: delimiter should be equal, expected '%c' got '%c'", n, c.expectedDelimiter, delimiter)
	}
}
