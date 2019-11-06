package com.photodispatcher.factory{
	import com.photodispatcher.context.Context;
	import com.photodispatcher.model.mysql.entities.AttrJsonMap;
	import com.photodispatcher.model.mysql.entities.DeliveryTypeDictionary;
	import com.photodispatcher.model.mysql.entities.MailPackage;
	import com.photodispatcher.model.mysql.entities.MailPackageBarcode;
	import com.photodispatcher.model.mysql.entities.MailPackageProperty;
	import com.photodispatcher.model.mysql.entities.Source;
	import com.photodispatcher.util.ArrayUtil;
	import com.photodispatcher.util.JsonUtil;
	import com.photodispatcher.util.StrUtil;
	
	import mx.collections.ArrayCollection;

	public class MailPackageBuilder{

		public static function build(source:int, raw:Object):MailPackage{
			if(!source || !raw ) return null;
			
			var result:MailPackage= new MailPackage();
			result.source=source;
			var src:Source= Context.getSource(source);
			if(src) result.source_name=src.name;
			var mpMap:Array= AttrJsonMap.getMailPackageJson();
			var mppMap:Array= AttrJsonMap.getMailPackagePropJson();
			
			//parce package
			var o:Object;
			var ajm:AttrJsonMap;
			var val:Object;
			var d:Date;
			for each(o in mpMap){
				ajm=o as AttrJsonMap;
				if(ajm){
					if(result.hasOwnProperty(ajm.field)){
						//params array
						val=JsonUtil.getRawVal(ajm.json_key, raw);
						if(val){
							if(ajm.field.indexOf('date')!=-1){
								//convert date
								d=JsonUtil.parseDate(val.toString());
								result[ajm.field]=d;
							}else{
								if(val is String){
									result[ajm.field]=StrUtil.siteCode2Char(val.toString());
								}else{
									result[ajm.field]=val;
								}
							}
						}
					}
				}
			}
			
			if(raw.hasOwnProperty('orders')){
				var orders_num:int=0;
				for(var s:String in raw.orders) orders_num++;
				result.orders_num= orders_num;
			}
			
			if(result.native_delivery_id) result.delivery_id=DeliveryTypeDictionary.translateDeliveryType(result.source, result.native_delivery_id);
			
			result.parseMessages();

			
			//build prorerties
			var props:Array=[];
			var prop:MailPackageProperty;
			for each(ajm in mppMap){
				val=JsonUtil.getRawVal(ajm.json_key, raw);
				if(val){
					prop= new MailPackageProperty();
					prop.source=source;
					prop.id=result.id;
					prop.property=ajm.field;
					prop.property_name=ajm.field_name;
					if(val is String){
						prop.value=StrUtil.siteCode2Char(val.toString());
					}else{
						prop.value=val.toString();
					}
					props.push(prop);
					//barcode?
					/*
					if(prop.property=='sl_delivery_code'){
						bar= new MailPackageBarcode();
						bar.source=source;
						bar.id=result.id;
						bar.barcode=prop.value;
						bar.bar_type=MailPackageBarcode.TYPE_SITE;
						barcodes.push(bar);
					}
					*/
				}
			}
			
			var barcodes:Array=[];
			var bar:MailPackageBarcode;
			var barObj:Object;
			var idx:int;

			//fill boxes (barcodes)
			if(raw.hasOwnProperty('boxes')){
				for each(barObj in raw.boxes){
					//if((barObj.hasOwnProperty('orderNumber') && barObj.orderNumber) || (barObj.hasOwnProperty('barcode') && barObj.barcode)){
						bar= new MailPackageBarcode();
						bar.source=source;
						bar.id=result.id;
						bar.bar_type=MailPackageBarcode.TYPE_SITE_BOX;
						if(barObj.hasOwnProperty('barcode')) bar.barcode=barObj.barcode;
						if(barObj.hasOwnProperty('id')) bar.box_id=barObj.id;
						if(barObj.hasOwnProperty('number')) bar.box_number=barObj.number;
						if(barObj.hasOwnProperty('weight')) bar.box_weight=barObj.weight;
						if(barObj.hasOwnProperty('orderId')) bar.box_orderId=barObj.orderId;
						if(barObj.hasOwnProperty('orderNumber')) bar.box_orderNumber=barObj.orderNumber;
						barcodes.push(bar); 
					//}
				}
			}

			//fill simple barcodes
			if(raw.hasOwnProperty('barcodes')){
				for each(barObj in raw.barcodes){
					if(barObj.hasOwnProperty('barcode') && barObj.barcode){
						//check & skip if same barcode exists
						idx= ArrayUtil.searchItemIdx('barcode',barObj.barcode,barcodes);
						if(idx==-1){
							//add
							bar= new MailPackageBarcode();
							bar.source=source;
							bar.id=result.id;
							bar.barcode=barObj.barcode;
							bar.bar_type=MailPackageBarcode.TYPE_SITE;
							if(barObj.hasOwnProperty('number')) bar.preorder_num=barObj.number;
							barcodes.push(bar); 
						}
					}
				}
			}


			
			result.properties= new ArrayCollection(props);
			result.barcodes= new ArrayCollection(barcodes);
			return result;
		}
		

	}
}